import {
	CanvasExt,
	View,
	RectCenter,
	RectTop,
	RectBottom,
	startOfMonth,
	endOfMonth,
	versionSortFunc,
	roundedRect,
	drawMonthDays,
	drawDailyBars,
	drawVScalesLeft,
	addLegend,
	drawLine,
	drawStacked,
	minMaxPercMulti,
	maxArrs,
	maxArrAbs,
	getDailyIncs,
	adjustZero,
	hoverSingle,
} from './utils.js'

function setupChart(setupFunc) {
	return function(wrap) {
		wrap.classList.add('ready')
		let canvasExt = CanvasExt.createIn(wrap, 'main-canvas')
		let redraw = setupFunc(wrap, canvasExt)
		window.addEventListener('resize', redraw)
		redraw()
	}
}
function regularRedraw(canvasExt, rects, redrawFunc) {
	return function() {
		canvasExt.resize()
		canvasExt.clear()
		for (let i = 0; i < rects.length; i++)
			rects[i].update(canvasExt.cssWidth, canvasExt.cssHeight)

		let rc = canvasExt.rc
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)
		redrawFunc(rc)
		rc.restore()
	}
}

let charts = {}

charts['node-activity-chart'] = setupChart(function(wrap, canvasExt) {
	let stamps = window.nodeActivityStamps
	let stampMargin = 3.5 * 60 * 1000 //3.5 minutes
	let zoomBoxTimeWidth = 24 * 3600 * 1000

	let monthMidStamp =
		stamps.length == 0 ? Date.now() : (stamps[0] + stamps[stamps.length - 1]) / 2
	let monthStart = startOfMonth(monthMidStamp)
	let monthEnd = endOfMonth(monthStart)

	let rect = new RectCenter({ left: 0, right: 0, top: 0, bottom: 10 })
	let view = new View({ startStamp: monthStart, endStamp: monthEnd, bottomValue: 0, topValue: 0 })
	let labelsRect = new RectBottom({ left: 0, right: 0, height: 4, bottom: 10 })
	let zoomBoxView = new View({
		startStamp: monthStart,
		endStamp: monthEnd,
		bottomValue: 0,
		topValue: 0,
	})
	let zoomBoxRect = new RectTop({ left: 0, right: 0, height: 32, top: 0 })
	let zoomBoxLabelsRect = new RectTop({ left: 0, right: 0, height: 4, top: 28 })

	function drawRegions(canvasExt, view, rect) {
		let height = rect.top + rect.height
		let rc = canvasExt.rc
		rc.fillStyle = '#EEE'
		rc.fillRect(0, 0, canvasExt.cssWidth, height)

		let i = 0
		while (i < stamps.length) {
			let stamp = stamps[i]
			let hasErr = stamp & 1
			//if (stamp < stampMin || stamp > stampMax) {i++; continue}

			let iNext = i
			while (++iNext < stamps.length) {
				let delta = stamps[iNext] - stamps[iNext - 1]
				if (delta > stampMargin * 2 || (stamps[iNext] & 1) != hasErr) break
			}
			let stampEnd = stamps[iNext - 1]

			let xStart =
				((stamp - stampMargin - view.startStamp) / view.duration) * canvasExt.cssWidth
			let xEnd =
				((stampEnd + stampMargin - view.startStamp) / view.duration) * canvasExt.cssWidth

			let minXWidth = 1 / canvasExt.pixelRatio
			if (xEnd - xStart < minXWidth) {
				let delta = (minXWidth - (xEnd - xStart)) / 2
				xStart -= delta
				xEnd += delta
			}

			rc.fillStyle = hasErr ? 'red' : 'limegreen'
			rc.fillRect(xStart, 0, xEnd - xStart, height)
			//rc.strokeRect(xStart, 0, xEnd - xStart, canvasExt.cssHeight)
			i = iNext
		}
	}

	const redraw = regularRedraw(canvasExt, [rect, labelsRect], function(rc) {
		drawRegions(canvasExt, view, rect)
		drawMonthDays(canvasExt, labelsRect, view)
		rc.strokeStyle = 'rgba(0,0,0,0.05)'
		rc.lineWidth = 0.5
		rc.strokeRect(0.5, 0.5, canvasExt.cssWidth - 1, rect.top + rect.height)
	})

	let zoomBoxCanvasExt = null
	function showZoomBox(x, y, e, touch) {
		if (zoomBoxCanvasExt === null) {
			zoomBoxCanvasExt = CanvasExt.createIn(wrap, 'zoom-box-canvas' + (touch ? ' touch' : ''))
		}
		let boxSize = zoomBoxCanvasExt.canvas.getBoundingClientRect()
		let pixRatio = window.devicePixelRatio
		let boxXMax = canvasExt.cssWidth - boxSize.width
		let boxX = Math.max(0, Math.min(x - boxSize.width / 2, boxXMax))
		boxX = Math.round(boxX * pixRatio) / pixRatio
		zoomBoxCanvasExt.canvas.style.transform = `translateX(${boxX}px)`

		zoomBoxCanvasExt.resize()
		zoomBoxCanvasExt.clear()
		zoomBoxRect.update(zoomBoxCanvasExt.cssWidth, zoomBoxCanvasExt.cssHeight)
		zoomBoxLabelsRect.update(zoomBoxCanvasExt.cssWidth, zoomBoxCanvasExt.cssHeight)

		let rc = zoomBoxCanvasExt.rc
		rc.save()
		rc.scale(zoomBoxCanvasExt.pixelRatio, zoomBoxCanvasExt.pixelRatio)

		rc.fillStyle = 'black'
		rc.fillRect(x - boxX - 0.5, zoomBoxCanvasExt.cssHeight - rect.bottom, 1, -rect.height)

		rc.fillStyle = 'rgba(255,255,255,0.5)'
		rc.fillRect(0, 0, zoomBoxCanvasExt.cssWidth, zoomBoxRect.height + 10 + 1)

		let timeW2 = zoomBoxTimeWidth / 2
		let pos = x / canvasExt.cssWidth
		let stamp = +monthStart + pos * (monthEnd - monthStart)
		zoomBoxView.updateStamps(stamp - timeW2, stamp + timeW2)

		rc.save()
		rc.beginPath()
		roundedRect(rc, 0.5, 0.5, zoomBoxRect.width - 1, zoomBoxRect.height, 2.5)
		rc.clip()
		drawRegions(zoomBoxCanvasExt, zoomBoxView, zoomBoxRect)
		rc.restore()
		rc.stroke()

		drawMonthDays(zoomBoxCanvasExt, zoomBoxLabelsRect, zoomBoxView)

		rc.restore()
	}
	function hideZoomBox() {
		if (zoomBoxCanvasExt !== null) {
			wrap.removeChild(zoomBoxCanvasExt.canvas)
			zoomBoxCanvasExt = null
		}
	}

	window.addEventListener('resize', hideZoomBox)
	hoverSingle({ elem: wrap, onHover: showZoomBox, onLeave: hideZoomBox })
	return redraw
})

charts['node-data-history-chart'] = setupChart(function(wrap, canvasExt) {
	let stamps = window.freeDataItems.stamps
	let diskValues = window.freeDataItems.freeDiskDeltas
	let bandValues = window.freeDataItems.freeBandwidthDeltas

	let efficiencies = new Float64Array(bandValues.length)
	for (let i = 0; i < bandValues.length; i++)
		efficiencies[i] = bandValues[i] == 0 ? 0 : (diskValues[i] / bandValues[i]) * 100

	let monthMidStamp =
		stamps.length == 0 ? Date.now() : (stamps[0] + stamps[stamps.length - 1]) / 2
	let monthStart = startOfMonth(monthMidStamp)
	let monthEnd = endOfMonth(monthStart)

	let [bottomValue, topValue] = minMaxPercMulti([diskValues, bandValues], 0.01)
	;[bottomValue, topValue] = adjustZero(bottomValue, topValue)
	if (bottomValue > 0) bottomValue = 0

	let rect = new RectCenter({ left: 0, right: 0, top: 1, bottom: 11 })
	let view = new View({
		startStamp: monthStart,
		endStamp: monthEnd,
		bottomValue: bottomValue,
		topValue: topValue,
	})

	var diskLabel = wrap.dataset.diskLabel
	var bandLabel = wrap.dataset.bandwidthLabel
	addLegend(wrap, [{ text: diskLabel, color: 'red' }, { text: bandLabel, color: 'green' }])

	function mbsLabel(value) {
		if (value == 0) return '0'
		let prefixes = ['b', 'Kib', 'Mib', 'Gib']
		let n = Math.min(Math.floor(Math.log2(Math.abs(value)) / 10), prefixes.length - 1)
		return (value / (1 << (n * 10))).toFixed(1) + ' ' + prefixes[n] + '/s'
	}
	return regularRedraw(canvasExt, [rect], function(rc) {
		drawMonthDays(canvasExt, rect, view, { vLinesColor: null, hLineColor: '#555' })

		drawLine(canvasExt, rect, view, stamps, diskValues, 'red')
		drawLine(canvasExt, rect, view, stamps, bandValues, 'green')

		drawVScalesLeft(canvasExt, rect, view, 'black', 'rgba(0,0,0,0.12)', mbsLabel)
	})
})

charts['node-data-history-coeff-chart'] = setupChart(function(wrap, canvasExt) {
	let stamps = window.freeDataItems.stamps
	let diskValues = window.freeDataItems.freeDiskDeltas
	let bandValues = window.freeDataItems.freeBandwidthDeltas

	let efficiencies = new Float64Array(bandValues.length)
	for (let i = 0; i < bandValues.length; i++)
		efficiencies[i] = bandValues[i] == 0 ? 0 : diskValues[i] / bandValues[i]

	let monthMidStamp =
		stamps.length == 0 ? Date.now() : (stamps[0] + stamps[stamps.length - 1]) / 2
	let monthStart = startOfMonth(monthMidStamp)
	let monthEnd = endOfMonth(monthStart)

	let [bottomValue, topValue] = minMaxPercMulti([efficiencies], 0.01, 3)
	if (bottomValue > 0) bottomValue = 0
	if (topValue < 1.2) topValue = 1.2

	let rect = new RectCenter({ left: 0, right: 0, top: 1, bottom: 11 })
	let view = new View({
		startStamp: monthStart,
		endStamp: monthEnd,
		bottomValue: bottomValue,
		topValue: topValue,
	})

	var coeffLabel = wrap.dataset.coeffLabel
	addLegend(wrap, [{ text: coeffLabel, color: 'blue' }])

	return regularRedraw(canvasExt, [rect], function(rc) {
		drawMonthDays(canvasExt, rect, view, { vLinesColor: null, hLineColor: '#555' })

		drawLine(canvasExt, rect, view, stamps, efficiencies, 'blue')

		drawVScalesLeft(canvasExt, rect, view, 'black', 'rgba(0,0,0,0.12)')
	})
})

charts['global-node-activity-counts-chart'] = setupChart(function(wrap, canvasExt) {
	let startTime = Date.parse(window.globalHistoryData.startTime)
	let endTime = Date.parse(window.globalHistoryData.endTime)
	let stamps = window.globalHistoryData.stamps //Uint32Array.from
	let countHours = window.globalHistoryData.countHours
	let hours = Object.keys(countHours).sort((a, b) => +a - +b)
	let revHours = hours.slice().reverse()

	let dailyIncs = getDailyIncs(startTime, endTime, stamps, countHours[24])

	let borderArrays = [countHours[hours[0]], countHours[hours[hours.length - 1]]]
	let [bottomValue, topValue] = minMaxPercMulti(borderArrays, 0.02)
	let d = (topValue - bottomValue) * 0.01
	bottomValue -= d
	topValue += d

	let barsTopValue = maxArrAbs(dailyIncs) * 1.5 //topValue - bottomValue

	let rect = new RectCenter({ left: 0, right: 0, top: 0, bottom: 11 })
	let view = new View({ startStamp: startTime, endStamp: endTime, bottomValue, topValue })
	let barsView = new View({
		startStamp: startTime,
		endStamp: endTime,
		bottomValue: 0,
		topValue: barsTopValue,
	})

	let hourLabel = wrap.dataset.hourLabel
	addLegend(wrap, revHours.map(h => ({ text: h + ' ' + hourLabel, color: hoursColor(h) })))

	function hoursColor(hours) {
		return 'hsl(240, 100%, ' + (50 + (1 - hours / 24) * 40) + '%)'
	}
	return regularRedraw(canvasExt, [rect], function(rc) {
		drawDailyBars(canvasExt, rect, barsView, dailyIncs, '#D0F7D0', '#F7D0D0', '#CCC')

		drawMonthDays(canvasExt, rect, view, { vLinesColor: null, hLineColor: '#555' })

		hours.forEach(hours => {
			let counts = countHours[hours]
			drawLine(canvasExt, rect, view, stamps, counts, hoursColor(hours), true)
		})

		drawVScalesLeft(canvasExt, rect, view, 'black', 'rgba(0,0,0,0.12)')
	})
})

charts['global-node-version-counts-chart'] = setupChart(function(wrap, canvasExt) {
	let startTime = Date.parse(window.globalHistoryData.startTime)
	let endTime = Date.parse(window.globalHistoryData.endTime)
	let stamps = window.globalHistoryData.stamps
	let countVersions = window.globalHistoryData.countVersions
	let versions = Object.keys(countVersions).sort(versionSortFunc)

	let topValue = maxArrs(Object.values(countVersions))

	let rect = new RectCenter({ left: 0, right: 0, top: 1, bottom: 11 })
	let view = new View({ startStamp: startTime, endStamp: endTime, bottomValue: 0, topValue })

	function pos(from, to, offset, x) {
		let mid = (to - from + offset * 2) / 2
		let clamped = Math.max(0, Math.min(x, to + offset) - (from - offset))
		return Math.min(1, (mid - Math.abs(clamped - mid)) / offset)
	}

	function versionColor(version) {
		let m = version.match(/v(\d+)\.(\d+)\.(\d+)/)
		if (m === null) return 'gray'
		let [, , b, c] = m
		let h = (b * 50) % 360
		let lk = 1 - pos(60, 180, 15, h) * 0.3 //диапазон 60-180 получается слишком ярким, затемняем его
		let l = 38 + Math.pow(((c * 7) % 20) * 1.4, lk)
		return `hsl(${h},100%,${l}%)`
	}

	addLegend(wrap, versions.map(v => ({ text: v, color: versionColor(v) })), 7)

	let stackAccum = new Float64Array(stamps.length)
	return regularRedraw(canvasExt, [rect], function(rc) {
		rc.fillStyle = 'rgba(255,255,255,0.05)'
		rc.fillRect(0, 0, canvasExt.cssWidth, canvasExt.cssHeight)

		drawMonthDays(canvasExt, rect, view, { vLinesColor: null, hLineColor: '#555' })

		stackAccum.fill(0)
		rc.globalCompositeOperation = 'destination-over'
		versions.forEach(version => {
			let counts = countVersions[version]
			drawStacked(canvasExt, rect, view, stamps, counts, versionColor(version), stackAccum)
		})
		rc.globalCompositeOperation = 'source-over'

		drawVScalesLeft(canvasExt, rect, view, 'black', 'rgba(0,0,0,0.12)')
	})
})

function makeDataDeltas(stamps, values) {
	let res = new Float64Array(values.length)
	for (let i = 0; i < values.length - 1; i++) {
		if (values[i + 1] != 0 && values[i] != 0) {
			res[i + 1] = ((values[i] - values[i + 1]) / (stamps[i + 1] - stamps[i])) * 8000
		}
	}
	// При переходе через месяц сбрасывается лимт трафика, и в первый час получается резкий всплеск.
	// Данных при этом ещё мало, и он нормально не образается. Так что просто затираем его.
	// TODO: как-то получше фильтровать такое
	if (res.length > 1) res[1] = res[2]
	if (res.length > 0) res[0] = res[1]
	return res
}

if ('freeDataItems' in window) {
	let di = window.freeDataItems
	di.freeDiskDeltas = makeDataDeltas(di.stamps, di.freeDisk)
	di.freeBandwidthDeltas = makeDataDeltas(di.stamps, di.freeBandwidth)
}

if ('nodeActivityStamps' in window) {
	let stamps = window.nodeActivityStamps
	for (let i = 0; i < stamps.length; i++) stamps[i] = (stamps[i] & ~1) * 1000 + (stamps[i] & 1)
}

document.querySelectorAll('.month-chart:not([data-off])').forEach(wrap => {
	charts[wrap.dataset.kind](wrap)
})
