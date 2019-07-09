import {
	CanvasExt,
	startOfMonth,
	endOfMonth,
	drawMonthDays,
	hoverSingle,
	roundedRect,
	minMaxPerc,
	minMaxPercMulti,
	iterateDays,
	maxArr,
	maxArrs,
	maxArrAbs,
	minArr,
	drawDailyBars,
	drawVScalesLeft,
	drawLegend,
	RectCenter,
	RectTop,
	RectBottom,
	View,
	drawLine,
	drawStacked,
	getDailyIncs,
	adjustZero,
} from './utils.js'

function setupActivityChart(wrap) {
	wrap.classList.add('ready')
	let canvasExt = CanvasExt.createIn(wrap, 'main-canvas')
	let stamps = window.nodeActivityStamps
	let stampMargin = 3.5 * 60 * 1000 //3.5 minutes
	let zoomBoxTimeWidth = 24 * 3600 * 1000

	for (let i = 0; i < stamps.length; i++) stamps[i] = (stamps[i] & ~1) * 1000 + (stamps[i] & 1)

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

	function redraw() {
		canvasExt.resize()
		canvasExt.clear()
		rect.update(canvasExt.cssWidth, canvasExt.cssHeight)
		labelsRect.update(canvasExt.cssWidth, canvasExt.cssHeight)

		let rc = canvasExt.rc
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)

		drawRegions(canvasExt, view, rect)
		drawMonthDays(canvasExt, labelsRect, view)
		rc.strokeStyle = 'rgba(0,0,0,0.05)'
		rc.lineWidth = 0.5
		rc.strokeRect(0.5, 0.5, canvasExt.cssWidth - 1, rect.top + rect.height)

		rc.restore()
	}

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

	window.addEventListener('resize', function() {
		hideZoomBox()
		redraw()
	})
	hoverSingle({ elem: wrap, onHover: showZoomBox, onLeave: hideZoomBox })
	redraw()
}

const setupDataHistoryChart = setupChart(function(wrap, canvasExt) {
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

	let [bottomValue, topValue] = minMaxPercMulti([diskValues, bandValues], 0.05)
	;[bottomValue, topValue] = adjustZero(bottomValue, topValue)

	let rect = new RectCenter({ left: 0, right: 0, top: 1, bottom: 11 })
	let view = new View({
		startStamp: monthStart,
		endStamp: monthEnd,
		bottomValue: bottomValue,
		topValue: topValue,
	})
	let effView = new View({
		startStamp: monthStart,
		endStamp: monthEnd,
		bottomValue: 0,
		topValue: 120,
	})

	function mbsLabel(value) {
		if (value == 0) return '0'
		let prefixes = ['b', 'Kib', 'Mib']
		let n = Math.min(Math.floor(Math.log2(Math.abs(value)) / 10), prefixes.length - 1)
		return (value / (1 << (n * 10))).toFixed(1) + ' ' + prefixes[n] + '/s'
	}
	return regularRedraw(canvasExt, [rect], function(rc) {
		drawMonthDays(canvasExt, rect, view, { vLinesColor: null, hLineColor: '#555' })

		drawLine(canvasExt, rect, view, stamps, diskValues, 'red')
		drawLine(canvasExt, rect, view, stamps, bandValues, 'green')
		//drawLine(canvasExt, rect, effView, stamps, efficiencies, 'blue')

		drawVScalesLeft(canvasExt, rect, view, 'black', 'rgba(0,0,0,0.12)', mbsLabel)

		drawLegend(canvasExt, rect, [
			{ text: 'диск', color: 'red' },
			{ text: 'трафик', color: 'green' },
		])
	})
})

const setupDataHistoryCoeffChart = setupChart(function(wrap, canvasExt) {
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

	let [bottomValue, topValue] = minMaxPercMulti([efficiencies], 0.05)
	if (bottomValue > 0) bottomValue = 0
	if (topValue < 1.2) topValue = 1.2

	let rect = new RectCenter({ left: 0, right: 0, top: 1, bottom: 11 })
	let view = new View({
		startStamp: monthStart,
		endStamp: monthEnd,
		bottomValue: bottomValue,
		topValue: topValue,
	})

	return regularRedraw(canvasExt, [rect], function(rc) {
		drawMonthDays(canvasExt, rect, view, { vLinesColor: null, hLineColor: '#555' })

		drawLine(canvasExt, rect, view, stamps, efficiencies, 'blue')

		drawVScalesLeft(canvasExt, rect, view, 'black', 'rgba(0,0,0,0.12)')

		drawLegend(canvasExt, rect, [{ text: 'диск/трафик', color: 'blue' }])
	})
})

function setupGlobalNodeActivityCountsChart(wrap) {
	wrap.classList.add('ready')
	let startTime = Date.parse(window.globalHistoryData.startTime)
	let endTime = Date.parse(window.globalHistoryData.endTime)
	let stamps = window.globalHistoryData.stamps //Uint32Array.from
	let countHours = window.globalHistoryData.countHours
	let hours = Object.keys(countHours).sort((a, b) => +a - +b)
	let revHours = hours.slice().reverse()

	let dailyIncs = getDailyIncs(startTime, endTime, stamps, countHours[24])

	let [bottomValue0, topValue0] = minMaxPerc(countHours[hours[0]], 0.02)
	let [bottomValue1, topValue1] = minMaxPerc(countHours[hours[hours.length - 1]], 0.02)
	let bottomValue = Math.min(bottomValue0, bottomValue1)
	let topValue = Math.max(topValue0, topValue1)
	let d = (topValue - bottomValue) * 0.01
	bottomValue -= d
	topValue += d

	let barsTopValue = maxArrAbs(dailyIncs) * 1.5 //topValue - bottomValue

	let canvasExt = CanvasExt.createIn(wrap, 'main-canvas')

	let rect = new RectCenter({ left: 0, right: 0, top: 0, bottom: 11 })
	let view = new View({ startStamp: startTime, endStamp: endTime, bottomValue, topValue })
	let barsRect = rect
	let barsView = new View({
		startStamp: startTime,
		endStamp: endTime,
		bottomValue: 0,
		topValue: barsTopValue,
	})

	function hoursColor(hours) {
		return 'hsl(240, 100%, ' + (50 + (1 - hours / 24) * 40) + '%)'
	}
	function redraw() {
		canvasExt.resize()
		canvasExt.clear()
		rect.update(canvasExt.cssWidth, canvasExt.cssHeight)

		let rc = canvasExt.rc
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)

		drawDailyBars(canvasExt, barsRect, barsView, dailyIncs, '#D0F7D0', '#F7D0D0', '#CCC')

		drawMonthDays(canvasExt, rect, view, { vLinesColor: null, hLineColor: '#555' })

		hours.forEach(hours => {
			let counts = countHours[hours]
			drawLine(canvasExt, rect, view, stamps, counts, hoursColor(hours))
		})

		drawVScalesLeft(canvasExt, rect, view, 'black', 'rgba(0,0,0,0.12)')

		drawLegend(canvasExt, rect, revHours.map(h => ({ text: h + ' ч', color: hoursColor(h) })))

		rc.restore()
	}

	window.addEventListener('resize', function() {
		redraw()
	})
	redraw()
}

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
const setupGlobalNodeVersionCountsChart = setupChart(function(wrap, canvasExt) {
	let startTime = Date.parse(window.globalHistoryData.startTime)
	let endTime = Date.parse(window.globalHistoryData.endTime)
	let stamps = window.globalHistoryData.stamps
	let countVersions = window.globalHistoryData.countVersions
	let versions = Object.keys(countVersions).sort()

	let topValue = maxArrs(Object.values(countVersions))

	let rect = new RectCenter({ left: 0, right: 0, top: 0, bottom: 11 })
	let view = new View({ startStamp: startTime, endStamp: endTime, bottomValue: 0, topValue })

	function versionColor(version) {
		let m = version.match(/v(\d+)\.(\d+)\.(\d+)/)
		if (m === null) return 'gray'
		let [, , b, c] = m
		return `hsl(${(b * 50) % 360},100%,${38 + ((c * 7) % 20) * 1.4}%)`
	}

	let stackAccum = new Int32Array(stamps.length)
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

		drawLegend(canvasExt, rect, versions.map(v => ({ text: v, color: versionColor(v) })), 3.5)
	})
})

document.querySelectorAll('.month-chart').forEach(wrap => {
	function makeDataDeltas(stamps, values) {
		let res = new Float64Array(values.length - 1)
		for (let i = 0; i < values.length - 1; i++) {
			if (values[i + 1] != 0 && values[i] != 0) {
				let d = Math.min(values.length - i - 1, 2)
				res[i] = ((values[i] - values[i + 1]) / (stamps[i + d] - stamps[i])) * 8000 * d
			}
		}
		res[res.length - 2] = res[res.length - 1]
		return res
	}

	if ('freeDataItems' in window) {
		let di = window.freeDataItems
		di.freeDiskDeltas = makeDataDeltas(di.stamps, di.freeDisk)
		di.freeBandwidthDeltas = makeDataDeltas(di.stamps, di.freeBandwidth)
	}

	switch (wrap.dataset.kind) {
		case 'node-activity-chart':
			setupActivityChart(wrap)
			break
		case 'node-data-history-chart':
			setupDataHistoryChart(wrap)
			break
		case 'node-data-history-coeff-chart':
			setupDataHistoryCoeffChart(wrap)
			break
		case 'global-node-activity-counts-chart':
			setupGlobalNodeActivityCountsChart(wrap)
			break
		case 'global-node-version-counts-chart':
			setupGlobalNodeVersionCountsChart(wrap)
			break
	}
})
