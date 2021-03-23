package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"storjnet/utils"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ansel1/merry"
	"storj.io/common/storj"
)

var ignoredSuffixes = []string{
	"connect: network is unreachable",
	"connect: no route to host",
	"connect: connection refused",
	"connect: connection reset by peer",
	"connect: broken pipe", //recovered?
	"read: connection reset by peer",
	"write: connection reset by peer",
	"write: broken pipe", //recovered?
	"rpccompat: EOF",
	"rpccompat: unexpected EOF",
	"rpccompat: tls: first record does not look like a TLS handshake",
	"rpccompat: tls: unsupported SSLv2 handshake received",
	"rpccompat: remote error: tls: error decoding message",
	"rpccompat: remote error: tls: unexpected message",
	"rpccompat: remote error: tls: internal error",
	"rpccompat: remote error: tls: handshake failure",
	"rpccompat: remote error: tls: protocol version not supported",
	"rpccompat: remote error: tls: insufficient security level", //recovered?
	"rpccompat: local error: tls: unexpected message",
	"rpccompat: context deadline exceeded",
	"i/o timeout",
}

func probeLoop(wg *sync.WaitGroup, sat *utils.Satellite, inAddrChan, failAddrChan, nodeAddrChan chan string) {
	wg.Add(1)
	defer wg.Done()

	id, err := storj.NodeIDFromString("1QzDKGHDeyuRxbvZhcwHU3syxTYtU1jHy5duAKuPxja3XC8ttk")
	if err != nil {
		log.Fatal(err)
	}

	for address := range inAddrChan {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := sat.DialAndClose(ctx, address, id)
		if err == nil {
			log.Fatal("probe completed without error, should not happen")
		} else {
			msg := err.Error()
			isKnown := false
			for _, suffix := range ignoredSuffixes {
				if strings.HasSuffix(msg, suffix) {
					isKnown = true
					break
				}
			}
			if isKnown {
				failAddrChan <- address
				continue
			}
			if strings.HasSuffix(msg, `peer ID did not match requested ID`) {
				// println("node: " + address)
				nodeAddrChan <- address
				continue
			}
			log.Fatal(address + ": " + merry.Details(err))
		}
	}
}

func addrSaveLoop(wg *sync.WaitGroup, addrChan chan string, fname string, count *int64) {
	wg.Add(1)
	defer wg.Done()

	f, err := os.Create(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for addr := range addrChan {
		_, err := f.WriteString(addr + "\n")
		if err != nil {
			log.Fatal(err)
		}
		if count != nil {
			*count++
		}
	}
}
func addAddrsToSet(set map[string]struct{}, fpath string) error {
	f, err := os.Open(fpath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return merry.Wrap(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		set[scanner.Text()] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		return merry.Wrap(err)
	}
	return nil
}
func countFileLines(fpath string) (int64, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return 0, merry.Wrap(err)
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	count := int64(0)
	lineSep := []byte{'\n'}

	for {
		c, err := f.Read(buf)
		count += int64(bytes.Count(buf[:c], lineSep))

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

func process(identityDir, inFpath string, port, linesCount int64, failsFpath, nodesFpath string, concurrency int64) error {
	portStr := strconv.FormatInt(port, 10)

	sat := &utils.Satellite{}
	if err := sat.SetUp(identityDir); err != nil {
		return merry.Wrap(err)
	}

	prevFailAddrSet := make(map[string]struct{})
	// err := addAddrsToSet(prevFailAddrSet, failsFpath)
	// if err != nil {
	// 	return merry.Wrap(err)
	// }
	err := addAddrsToSet(prevFailAddrSet, failsFpath+".tmp")
	if err != nil {
		return merry.Wrap(err)
	}

	prevNodeAddrSet := make(map[string]struct{})
	err = addAddrsToSet(prevNodeAddrSet, nodesFpath)
	if err != nil {
		return merry.Wrap(err)
	}
	err = addAddrsToSet(prevNodeAddrSet, nodesFpath+".tmp")
	if err != nil {
		return merry.Wrap(err)
	}

	var inFile *os.File
	if inFpath == "-" {
		inFile = os.Stdin
	} else {
		inFile, err = os.Open(inFpath)
		if err != nil {
			return merry.Wrap(err)
		}
		defer inFile.Close()
	}

	lineNum := int64(0)
	nodesCount := int64(0)

	if linesCount == 0 {
		stat, err := os.Stat(inFpath)
		if err != nil {
			return merry.Wrap(err)
		}
		if stat.Mode().IsRegular() {
			linesCount, err = countFileLines(inFpath)
			if err != nil {
				return merry.Wrap(err)
			}
		}
	}

	inAddrChan := make(chan string, 10)
	failAddrChan := make(chan string, 10)
	nodeAddrChan := make(chan string, 10)
	probeWg := &sync.WaitGroup{}
	saveWg := &sync.WaitGroup{}
	for i := int64(0); i < concurrency; i++ {
		go probeLoop(probeWg, sat, inAddrChan, failAddrChan, nodeAddrChan)
	}
	go addrSaveLoop(saveWg, failAddrChan, failsFpath+".tmp", nil)
	go addrSaveLoop(saveWg, nodeAddrChan, nodesFpath+".tmp", &nodesCount)

	go func() {
		prevStamp := time.Now()
		prevLineNum := lineNum
		interval := 5 * time.Second
		for {
			stamp := time.Now()
			lineNum := lineNum
			speed := float64(lineNum-prevLineNum) * float64(time.Second) / float64(stamp.Sub(prevStamp))
			est := int64(0)
			if lineNum > 0 {
				est = linesCount * nodesCount / lineNum
			}
			log.Printf("line: %d, %.02f l/s, nodes: %d found, %d estimated", lineNum, speed, nodesCount, est)
			prevStamp = stamp
			prevLineNum = lineNum
			time.Sleep(interval - time.Duration(stamp.UnixNano())%interval)
		}
	}()

	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		address := scanner.Text() + ":" + portStr

		if _, ok := prevFailAddrSet[address]; ok {
			failAddrChan <- address
		} else if _, ok := prevNodeAddrSet[address]; ok {
			nodeAddrChan <- address
		} else {
			inAddrChan <- address
		}

		lineNum++
	}
	if err := scanner.Err(); err != nil {
		return merry.Wrap(err)
	}

	close(inAddrChan)
	probeWg.Wait()
	close(failAddrChan)
	close(nodeAddrChan)
	saveWg.Wait()

	if err := os.Rename(failsFpath+".tmp", failsFpath); err != nil {
		return merry.Wrap(err)
	}
	if err := os.Rename(nodesFpath+".tmp", nodesFpath); err != nil {
		return merry.Wrap(err)
	}

	log.Printf("done, %d line(s) processed, %d node(s) found", lineNum, nodesCount)
	return nil
}

func main() {
	identityDir := flag.String("identity-dir", "identity", "path identity directory")
	inFpath := flag.String("in-file", "-", "path to file with IP addresses newline-separated list (stdin by default)")
	port := flag.Int64("port", 28967, "port of node IPs from in-file")
	linesCount := flag.Int64("total-addrs", 0, "lines count in input file; used for nodes count estimation; obtained from in-file by default")
	failsFpath := flag.String("fails-file", "fail_addrs.txt", "file with failed addr:port list from pervious pass, will be updated")
	nodesFpath := flag.String("nodes-file", "node_addrs.txt", "file with node addr:port list from pervious pass, will be updated")
	concurrency := flag.Int64("concurrency", 128, "roughly equals to requests_per_second / 2")
	flag.Parse()

	if err := process(*identityDir, *inFpath, *port, *linesCount, *failsFpath, *nodesFpath, *concurrency); err != nil {
		log.Fatal(merry.Details(err))
	}
}
