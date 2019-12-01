export class CanvasExt {
	constructor() {
		this.canvas = null
		this.rc = null
		this.pixelRatio = 1
		this.cssWidth = 0
		this.cssHeight = 0
		this.setRef = ref => {
			this.canvas = ref
			this.rc = ref === null ? null : ref.getContext('2d')
		}
	}
	_setRealSize(w, h) {
		if (this.canvas.width == w && this.canvas.height == h) return false
		this.canvas.width = w
		this.canvas.height = h
		return true
	}
	created() {
		return this.canvas !== null
	}
	resize() {
		let rect = this.canvas.getBoundingClientRect()
		let { width: cssWidth, height: cssHeight } = rect
		let dpr = window.devicePixelRatio
		this.pixelRatio = dpr
		this.cssWidth = cssWidth
		this.cssHeight = cssHeight
		let width = Math.round(rect.right * dpr) - Math.round(rect.left * dpr)
		let height = Math.round(rect.bottom * dpr) - Math.round(rect.top * dpr)
		return this._setRealSize(width, height)
	}
	clear() {
		this.rc.clearRect(0, 0, this.canvas.width, this.canvas.height)
	}
}

export class RectCenter {
	constructor({ left, right, top, bottom }) {
		this.left = left
		this.right = right
		this.top = top
		this.bottom = bottom
		this.width = 0
		this.height = 0
	}

	update(outerWidth, outerHeight) {
		this.width = outerWidth - this.left - this.right
		this.height = outerHeight - this.top - this.bottom
	}
}

export class RectBottom {
	constructor({ left, right, bottom, height }) {
		this.left = left
		this.right = right
		this.top = 0
		this.bottom = bottom
		this.width = 0
		this.height = height
	}

	update(outerWidth, outerHeight) {
		this.width = outerWidth - this.left - this.right
		this.top = outerHeight - this.bottom - this.height
	}
}

export class RectTop {
	constructor({ left, right, top, height }) {
		this.left = left
		this.right = right
		this.top = top
		this.bottom = 0
		this.width = 0
		this.height = height
	}

	update(outerWidth, outerHeight) {
		this.width = outerWidth - this.left - this.right
		this.bottom = outerHeight - this.top - this.height
	}
}

export class View {
	constructor({ startStamp, endStamp, bottomValue, topValue }) {
		this.startStamp = startStamp
		this.endStamp = endStamp
		this.bottomValue = bottomValue
		this.topValue = topValue
		this.duration = this.endStamp - this.startStamp
		this.height = this.topValue - this.bottomValue
	}

	updateStamps(startStamp, endStamp) {
		this.startStamp = startStamp
		this.endStamp = endStamp
		this.duration = this.endStamp - this.startStamp
	}
}

function stamp2x(rect, view, stamp) {
	return rect.left + ((stamp - view.startStamp) / view.duration) * rect.width
}
function value2y(rect, view, value) {
	return rect.top + rect.height - ((value - view.bottomValue) / view.height) * rect.height
}

export function drawPingLine(canvasExt, rect, view, pings, startStamp, itemWidth, color) {
	let rc = canvasExt.rc
	rc.beginPath()
	let started = false
	for (let i = 0; i < pings.length; i++) {
		let value = pings[i]
		let ping = value % 2000
		if (ping > 1) {
			let timeHint = Math.floor(value / 2000) * 4
			let time = startStamp + i * itemWidth + timeHint
			let x = stamp2x(rect, view, time)
			let y = value2y(rect, view, ping)
			if (started) {
				rc.lineTo(x, y)
			} else {
				rc.moveTo(x, y)
				started = true
			}
		} else {
			started = false
		}
	}
	rc.lineJoin = 'bevel'
	rc.strokeStyle = color
	rc.stroke()
}

export function forEachPingRegion(rect, view, pings, startStamp, itemWidth, needKind, func) {
	let prevKind = null
	let prevKindX = 0
	let lastX = 0
	for (let i = 0; i < pings.length; i++) {
		let value = pings[i]
		let ping = value % 2000
		let timeHint = Math.floor(value / 2000) * 4
		let time = startStamp + i * itemWidth + timeHint
		let x = stamp2x(rect, view, time)

		let kind = Math.min(2, ping)
		if (kind != prevKind) {
			if (prevKind == needKind) {
				func(prevKindX, x)
			}
			prevKind = kind
			prevKindX = x
		}
		lastX = x
	}
	if (prevKind == needKind) {
		func(prevKindX, lastX)
	}
}
/*
function forEachPing(pings, func) {
	for (let i = 0; i < pings.length; i++) {
		let value = pings[i]
		let ping = value % 2000
		let timeHint = Math.floor(value / 2000) * 4
		let time = i * 60 + timeHint
		let x = time / 60 / 18
		let y = ping / 20
		func(x, y, time, ping)
	}
}
function forEachPingReduced(pings, func) {
	let n = 60
	for (let j = 0; j < pings.length; j += n) {
		let minPing = Infinity
		let maxPing = -Infinity
		let minTime = null
		let maxTime = null
		for (let i = j; i < Math.min(j + n, pings.length); i++) {
			let value = pings[i]
			let ping = value % 2000
			if (ping > 1) {
				let timeHint = Math.floor(value / 2000) * 4
				let time = i * 60 + timeHint
				if (ping < minPing) {
					minPing = ping
					minTime = time
				}
				if (ping > maxPing) {
					maxPing = ping
					maxTime = time
				}
			}
		}
		if (minTime > maxTime) {
			;[minTime, maxTime] = [maxTime, minTime]
		}
		func(minTime / 60 / 18, minPing / 20, minTime, minPing)
		func(maxTime / 60 / 18, maxPing / 20, maxTime, maxPing)
	}
}
function forEachPingAvg(pings, func) {
	let n = 30
	for (let j = 0; j < pings.length; j += n) {
		let pingSum = 0
		let timeSum = 0
		let count = 0
		for (let i = j; i < Math.min(j + n, pings.length); i++) {
			let value = pings[i]
			let ping = value % 2000
			if (ping > 1) {
				let timeHint = Math.floor(value / 2000) * 4
				let time = i * 60 + timeHint
				pingSum += ping
				timeSum += time
				count++
			}
		}
		if (count > 0)
			func(
				timeSum / count / 60 / 18,
				48 * 1.5 - pingSum / count / 20,
				timeSum / count,
				pingSum / count,
			)
	}
}
*/

function* iterateDays(startDate, endDate, step = 1) {
	if (step < 1) step = 1
	startDate = new Date(startDate)
	startDate.setHours(0, 0, 0, 0)

	let dayDate = new Date(startDate)
	for (let dayNum = 0; ; dayNum += step) {
		let nextDayDate = new Date(startDate)
		nextDayDate.setDate(nextDayDate.getDate() + dayNum + 1)

		yield [dayNum, dayDate, nextDayDate]

		if (nextDayDate >= endDate) break
		dayDate = nextDayDate
	}
}

export function drawMonthDays(canvasExt, rect, view, params = {}) {
	let { textColor = 'black', vLinesColor = null, hLineColor = '#555' } = params

	let rc = canvasExt.rc
	rc.fillStyle = 'black'
	rc.textAlign = 'center'
	rc.textBaseline = 'top'

	let labelWidth = rc.measureText('30').width * 1.5
	let approxNumDays = view.duration / (24 * 3600 * 1000)
	let maxLabels = rect.width / labelWidth
	let step = Math.ceil(approxNumDays / maxLabels)

	for (let [, dayDate, nextDayDate] of iterateDays(view.startStamp, view.endStamp, step)) {
		let y = rect.top + rect.height
		// drawHours(
		// 	canvasExt,
		// 	view.startStamp,
		// 	view.endStamp,
		// 	dayDate,
		// 	nextDayDate,
		// 	y,
		// 	rect.height - 2,
		// )
		let x = stamp2x(rect, view, dayDate)
		if (vLinesColor !== null) {
			rc.fillStyle = vLinesColor
			rc.fillRect(x, y, 1, -rect.height)
		}
		rc.fillStyle = textColor
		rc.fillText(dayDate.getDate(), x, y + (hLineColor === null ? 2 : 3))
	}

	if (hLineColor !== null) {
		rc.fillStyle = hLineColor
		rc.fillRect(rect.left, rect.top + rect.height - 0.5, rect.width, 1)
	}
}

function roundLabelValues(bottomValue, topValue, roundN) {
	let k = Math.pow(10, roundN)

	bottomValue = Math.ceil(bottomValue / k) * k
	topValue = Math.floor(topValue / k) * k
	let midValue = (topValue + bottomValue) / 2

	let height = topValue - bottomValue
	let bottomK = bottomValue / height
	let topK = topValue / height

	if (bottomK < -0.2 && topK > 0.2) {
		// если мин и макс в 20%+ от нуля, отображаем ноль
		midValue = 0
		if (bottomK < -0.4 && topK > 0.4) {
			// если они в 40%+ от нуля (ноль почти в середине),
			// корректируем одно из значений так, чтоб ноль был ровно между значениями
			let delta = Math.min(topValue, -bottomValue)
			topValue = delta
			bottomValue = -delta
		}
	} else if (bottomK < 0 && bottomK > -0.2 && topK > 0) {
		// если нижняя граница немного ниже нуля, сдвигаем её к нулю
		bottomValue = 0
		midValue = topValue / 2
	}
	return [bottomValue, midValue, topValue]
}
export function drawLabeledVScaleLeftLine(rc, rect, view, value, textColor, lineColor, roundN, textFunc) {
	let text = textFunc === null ? value.toFixed(Math.max(0, -roundN)) : textFunc(value)
	let lineY = ((view.topValue - value) / view.height) * rect.height
	rc.strokeStyle = 'rgba(255,255,255,0.75)'
	rc.lineWidth = 2
	rc.strokeText(text, rect.left + 2, rect.top + lineY)
	rc.fillStyle = textColor
	rc.fillText(text, rect.left + 2, rect.top + lineY)
	if (lineColor !== null) {
		rc.fillStyle = lineColor
		rc.fillRect(rect.left, rect.top + lineY - 0.5, rect.width, 1)
	}
}
export function drawVScalesLeft(canvasExt, rect, view, textColor, lineColor, textFunc = null) {
	let rc = canvasExt.rc
	let roundN = Math.floor(Math.log10(view.topValue - view.bottomValue) - 1)
	let topOffset = (view.height / rect.height) * 11 //font height
	let values = roundLabelValues(view.bottomValue, view.topValue - topOffset, roundN)
	rc.textAlign = 'left'
	rc.textBaseline = 'bottom'
	for (let i = 0; i < values.length; i++)
		drawLabeledVScaleLeftLine(rc, rect, view, values[i], textColor, lineColor, roundN, textFunc)
}
