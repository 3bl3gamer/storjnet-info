import {
	CanvasExt,
	startOfMonth,
	endOfMonth,
	drawMonthDays,
	hoverSingle,
	roundedRect,
	minMaxPerc,
	iterateDays,
	maxAbs,
	drawDailyBars,
	drawVScalesLeft,
	drawLegend,
	RectCenter,
	RectTop,
	RectBottom,
	View,
	drawLine,
	getDailyIncs,
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

function setupGlobalNodeActivityCountsChart(wrap) {
	wrap.classList.add('ready')
	let startTime = Date.parse(window.globalHistoryData.startTime)
	let endTime = Date.parse(window.globalHistoryData.endTime)
	let stamps = window.globalHistoryData.stamps //Uint32Array.from
	let countHours = window.globalHistoryData.countHours
	let hours = Object.keys(countHours).sort((a, b) => +a - +b)
	let revHours = hours.slice().reverse()

	for (let i = 0; i < stamps.length; i++) stamps[i] *= 1000

	let dailyIncs = getDailyIncs(startTime, endTime, stamps, countHours[24])

	let [bottomValue0, topValue0] = minMaxPerc(countHours[hours[0]], 0.02)
	let [bottomValue1, topValue1] = minMaxPerc(countHours[hours[hours.length - 1]], 0.02)
	let bottomValue = Math.min(bottomValue0, bottomValue1)
	let topValue = Math.max(topValue0, topValue1)
	let d = (topValue - bottomValue) * 0.1
	bottomValue -= d
	topValue += d

	let barsTopValue = maxAbs(dailyIncs) * 1.5 //topValue - bottomValue

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

document.querySelectorAll('.month-chart').forEach(wrap => {
	switch (wrap.dataset.kind) {
		case 'node-activity-chart':
			setupActivityChart(wrap)
			break
		case 'global-node-activity-counts-chart':
			setupGlobalNodeActivityCountsChart(wrap)
			break
	}
})
