import {
	CanvasExt,
	startOfMonth,
	endOfMonth,
	drawMonthDays,
	hoverSingle,
	roundedRect,
} from './utils.js'

function setupActivityChart(wrap) {
	let canvasExt = CanvasExt.createIn(wrap, 'main-canvas')
	let stamps = window.nodeActivityStamps
	let stampMargin = 3 * 60 //3 minutes
	let hScalesHeight = 10
	let zoomBoxTimeWidth = 24 * 3600

	let monthMidStamp =
		stamps.length == 0 ? Date.now() : ((stamps[0] + stamps[stamps.length - 1]) / 2) * 1000
	let monthStart = startOfMonth(monthMidStamp)
	let monthEnd = endOfMonth(monthStart)

	function drawRegions(canvasExt, stampMin, stampMax, height) {
		let rc = canvasExt.rc
		rc.fillStyle = '#EEE'
		rc.fillRect(0, 0, canvasExt.cssWidth, height)

		let stampWidth = stampMax - stampMin
		let i = 0
		while (i < stamps.length) {
			let stamp = stamps[i]
			let hasErr = stamp & 1
			//if (stamp < stampMin || stamp > stampMax) {i++; continue}

			let iNext = i
			while (++iNext < stamps.length) {
				let delta = stamps[iNext] - stamps[iNext - 1]
				if (delta > 7 * 60 || (stamps[iNext] & 1) != hasErr) break
			}
			let stampEnd = stamps[iNext - 1]

			let xStart = ((stamp - stampMargin - stampMin) / stampWidth) * canvasExt.cssWidth
			let xEnd = ((stampEnd + stampMargin - stampMin) / stampWidth) * canvasExt.cssWidth

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
		let rc = canvasExt.rc
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)

		let stampMin = monthStart.getTime() / 1000 //stamps[0] //
		let stampMax = monthEnd.getTime() / 1000 //stamps[stamps.length - 1] //
		drawRegions(canvasExt, stampMin, stampMax, canvasExt.cssHeight - hScalesHeight)
		drawMonthDays(
			canvasExt,
			stampMin * 1000,
			stampMax * 1000,
			canvasExt.cssHeight - hScalesHeight,
			4,
		)
		rc.strokeStyle = 'rgba(0,0,0,0.05)'
		rc.lineWidth = 0.5
		rc.strokeRect(0.5, 0.5, canvasExt.cssWidth - 1, canvasExt.cssHeight - hScalesHeight)

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
		let rc = zoomBoxCanvasExt.rc
		rc.save()
		rc.scale(zoomBoxCanvasExt.pixelRatio, zoomBoxCanvasExt.pixelRatio)

		rc.fillStyle = 'black'
		rc.fillRect(x - boxX - 0.5, zoomBoxCanvasExt.cssHeight - hScalesHeight, 1, -30)

		rc.fillStyle = 'rgba(255,255,255,0.5)'
		rc.fillRect(0, 0, zoomBoxCanvasExt.cssWidth, 32 + hScalesHeight + 1)

		let timeW2 = zoomBoxTimeWidth / 2
		let pos = x / canvasExt.cssWidth
		let stamp = (+monthStart + pos * (monthEnd - monthStart)) / 1000
		rc.save()
		rc.beginPath()
		roundedRect(rc, 0.5, 0.5, zoomBoxCanvasExt.cssWidth - 1, 31, 2.5)
		rc.clip()
		drawRegions(zoomBoxCanvasExt, stamp - timeW2, stamp + timeW2, 32)
		rc.restore()
		rc.stroke()

		drawMonthDays(zoomBoxCanvasExt, (stamp - timeW2) * 1000, (stamp + timeW2) * 1000, 32, 6)

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

document.querySelectorAll('.month-chart').forEach(wrap => {
	if (wrap.dataset.kind == 'node-activity-chart') setupActivityChart(wrap)
})
