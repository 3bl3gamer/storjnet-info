# while true; do timeout --kill-after=5s --verbose 4h sh run_node_with_stat.sh; sleep 1; done
~/storj_repo/cmd/storagenode/storagenode run \
  --identity-dir ./identity2 \
  --config-dir ./config2 \
  --kademlia.operator.wallet=0x2d427Cf50166123aB7aCfB37BAa2d59729Cd2D12 \
  --kademlia.operator.email=my@mail.com \
  --kademlia.bootstrap-addr=bootstrap.storj.io:8888 \
  --kademlia.external-address=51.15.216.154:28967 \
  --log.level debug \
  --log.stack \
  --log.caller \
  --log.development \
2>&1 | tee -a log.txt | grep 'NODE:KAD:' | grep -oP '{.*' | \
  ../storj3stat/storj3stat run \
    --start-delay 1 \
    --kad-save-chunk-size 100 \
    --self-fetch-routines 32 2>&1 | tee -a log_all.txt
