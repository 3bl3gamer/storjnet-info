export class CanvasExt {
	constructor(canvas) {
		this.canvas = canvas
		this.rc = this.canvas.getContext('2d')
		this.pixelRatio = 1
		this.cssWidth = this.canvas.width
		this.cssHeight = this.canvas.height
	}
	static createIn(wrap, className = '') {
		let canvas = document.createElement('canvas')
		canvas.className = className
		wrap.appendChild(canvas)
		return new CanvasExt(canvas)
	}
	_setRealSize(w, h) {
		if (this.canvas.width == w && this.canvas.height == h) return false
		this.canvas.width = w
		this.canvas.height = h
		return true
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

export function startOfMonth(date) {
	let newDate = new Date(date)
	newDate.setUTCHours(0, 0, 0, 0)
	newDate.setUTCDate(1)
	return newDate
}
export function endOfMonth(date) {
	date = startOfMonth(date)
	date.setUTCMonth(date.getUTCMonth() + 1)
	return date
}

function version2num(v) {
	let m = v.match(/v(\d+)\.(\d+)\.(\d+)/)
	if (m === null) return Infinity
	return m[1] * 10000 + m[2] * 100 + +m[3]
}
export function versionSortFunc(a, b) {
	return version2num(a) - version2num(b)
}

export function getIndexBinary(values, x, indexFrom = 0, indexTo = null) {
	if (indexTo === null) indexTo = values.length
	while (indexFrom < indexTo - 1) {
		let indexMid = (indexFrom + indexTo) >> 1
		if (x < values[indexMid]) {
			indexTo = indexMid
		} else {
			indexFrom = indexMid
		}
	}
	return indexFrom
}

export function minMaxPerc(values, perc, bottomCutValue, topCutValue) {
	// если все значения пропущены, считать нечего
	let skipValue = 0
	let valuesCount = 0
	for (let i = 0; i < values.length; i++)
		if (values[i] != skipValue && values[i] >= bottomCutValue && values[i] <= topCutValue)
			valuesCount++
	if (valuesCount == 0) return [0, 0]

	// ман/макс по текущим значениям (с обрезкой)
	let max = -Infinity
	let min = Infinity
	for (let i = 0; i < values.length; i++) {
		let v = values[i]
		if (v != skipValue && values[i] >= bottomCutValue && values[i] <= topCutValue) {
			if (v > max) max = v
			if (v < min) min = v
		}
	}

	// подсчёт повторяемости значений (округлённых, в counts будет подобие гистограммы)
	let counts = new Uint32Array(100)
	for (let i = 0; i < values.length; i++) {
		let v = values[i]
		if (v != skipValue && values[i] >= bottomCutValue && values[i] <= topCutValue)
			counts[(((v - min) / (max - min)) * (counts.length - 1)) | 0]++
	}

	// "зачистка" гистограммы: удаление с неё редких всплесков
	let n = 3
	let sum = 0
	for (let i = 0; i < n; i++) sum += counts[i]
	for (let i = 0; i < counts.length; i++) {
		if (i + n < counts.length - 1) sum += counts[i + n]
		if (counts[i] > 0 && sum < values.length*0.1) {
			counts[i]--
			sum--
		}
		if (i - n >= 0) sum -= counts[i - n]
	}

	// поиск нового мин/макса (сверху и снизу пропускается perc*100% значений)
	let thresh = values.length * perc
	let minPerc = min
	let maxPerc = max
	for (let count = 0, i = 0; i < counts.length; i++) {
		count += counts[i]
		if (count > thresh) {
			let d = (count - thresh) / counts[i]
			minPerc = min + ((i + 1 - d) / (counts.length - 1)) * (max - min)
			break
		}
	}
	for (let count = 0, i = counts.length - 1; i >= 0; i--) {
		count += counts[i]
		if (count > thresh) {
			let d = (count - thresh) / counts[i]
			maxPerc = min + ((i + d) / (counts.length - 1)) * (max - min)
			break
		}
	}

	// немножко расширяем интервал
	let delta = (maxPerc - minPerc) * 0.2
	return minMaxArrCut(values, minPerc - delta, maxPerc + delta, skipValue)
}
export function minMaxPercMulti(arrays, perc, passes = 2) {
	let min = Infinity
	let max = -Infinity
	for (let i = 0; i < arrays.length; i++) {
		let bottom = -Infinity
		let top = Infinity
		for (let j = 0; j < passes; j++) {
			;[bottom, top] = minMaxPerc(arrays[i], perc, bottom, top)
		}
		if (bottom < min) min = bottom
		if (top > max) max = top
	}
	return [min, max]
}
export function minMaxArrCut(values, bottomValue, topValue, skipValue = 0) {
	let max = -Infinity
	let min = Infinity
	for (let i = 0; i < values.length; i++) {
		let v = values[i]
		if (v != skipValue && v >= bottomValue && v <= topValue) {
			if (v > max) max = v
			if (v < min) min = v
		}
	}
	return [min, max]
}

export function maxArr(values) {
	let max = -Infinity
	for (let i = 0; i < values.length; i++) {
		let v = values[i]
		if (v > max) max = v
	}
	return max
}
export function minArr(values) {
	let min = Infinity
	for (let i = 0; i < values.length; i++) {
		let v = values[i]
		if (v == 0) continue
		if (v < min) min = v
	}
	return min
}
export function maxArrs(arrays) {
	if (arrays.length == 0) return -Infinity
	let accum = new Float64Array(arrays[0].length)
	for (let i = 0; i < arrays.length; i++) {
		let values = arrays[i]
		for (let j = 0; j < values.length; j++) {
			accum[j] += values[j]
		}
	}
	return maxArr(accum)
}
export function maxArrAbs(values) {
	let max = -Infinity
	for (let i = 0; i < values.length; i++) {
		let v = Math.abs(values[i])
		if (v > max) max = v
	}
	return max
}

export function adjustZero(bottomValue, topValue) {
	let height = topValue - bottomValue
	let heightK = bottomValue / height
	if (heightK > -0.05 && heightK < 0.1) return [0, topValue]
	return [bottomValue, topValue]
}

export function getDailyIncs(startDate, endDate, stamps, values) {
	let dailyIncs = []
	for (let [, dayDate, nextDayDate] of iterateDays(startDate, endDate)) {
		let iFrom = getIndexBinary(stamps, dayDate)
		let iTo = getIndexBinary(stamps, nextDayDate)
		dailyIncs.push(values[iTo] - values[iFrom])
	}
	return dailyIncs
}

export function roundedRectLeft(rc, x, y, h, r) {
	rc.lineTo(x + r, y + h)
	rc.arcTo(x, y + h, x, y + h - r, r)
	rc.lineTo(x, y + y + r)
	rc.arcTo(x, y, x + r, y, r)
}
export function roundedRectRight(rc, x, y, h, r) {
	rc.lineTo(x - r, y)
	rc.arcTo(x, y, x, y + r, r)
	rc.lineTo(x, y + h - r)
	rc.arcTo(x, y + h, x - r, y + h, r)
}
export function roundedRect(rc, x, y, w, h, r) {
	rc.moveTo(x + w - r, y + h)
	roundedRectLeft(rc, x, y, h, r)
	roundedRectRight(rc, x + w, y, h, r)
}

export function* iterateDays(startDate, endDate, step = 1) {
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
	let { textColor = 'black', vLinesColor = 'black', hLineColor = null } = params

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
		drawHours(
			canvasExt,
			view.startStamp,
			view.endStamp,
			dayDate,
			nextDayDate,
			y,
			rect.height - 2,
		)
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
function drawHours(canvasExt, dateDrawFrom, dateDrawTo, curDayDate, nextDayDate, y, markHeight) {
	let rc = canvasExt.rc

	let labelWidth = rc.measureText('12:00').width * 1.5
	let dayWidth = (canvasExt.cssWidth * (nextDayDate - curDayDate)) / (dateDrawTo - dateDrawFrom)
	let maxLabels = dayWidth / labelWidth
	let step = [1, 2, 3, 4, 6, 12].find(x => x > 24 / maxLabels)
	if (step == null) return

	rc.strokeStyle = 'rgba(255,255,255,0.3)'
	rc.lineWidth = 2.5
	for (let hourNum = step; ; hourNum += step) {
		let curDate = new Date(curDayDate)
		curDate.setHours(curDate.getHours() + hourNum)
		if (curDate >= nextDayDate) break
		let x = ((curDate - dateDrawFrom) / (dateDrawTo - dateDrawFrom)) * canvasExt.cssWidth
		rc.fillStyle = 'black'
		rc.fillRect(x, y, 1, -markHeight)
		rc.strokeText(curDate.getHours() + ':00', x, y + 2)
		rc.fillStyle = '#777'
		rc.fillText(curDate.getHours() + ':00', x, y + 2)
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
function drawLabeledVScaleLeftLine(rc, rect, view, value, textColor, lineColor, roundN, textFunc) {
	let text = textFunc === null ? value.toFixed(Math.max(0, -roundN)) : textFunc(value)
	let lineY = ((view.topValue - value) / view.height) * rect.height
	rc.strokeStyle = 'rgba(255,255,255,0.75)'
	rc.lineWidth = 2
	rc.strokeText(text, rect.left + 2, rect.top + lineY)
	rc.fillStyle = textColor
	rc.fillText(text, rect.left + 2, rect.top + lineY)
	rc.fillStyle = lineColor
	rc.fillRect(rect.left, rect.top + lineY - 0.5, rect.width, 1)
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

export function drawLegend(canvasExt, rect, items, lineWidth = 0.5) {
	let rc = canvasExt.rc
	let x = rect.left + 48
	let y = rect.top + 1
	let lineLength = 12
	let lineSep = 3
	let itemSep = 8

	let totalWidth = items.length * (lineLength + lineSep) + (items.length - 1) * itemSep
	for (let i = 0; i < items.length; i++) totalWidth += rc.measureText(items[i].text).width
	rc.fillStyle = 'rgba(255,255,255,0.5)'
	rc.fillRect(x - 2, y - 1, totalWidth + 4, 8 + 2)

	rc.textAlign = 'left'
	rc.textBaseline = 'top'
	for (let i = 0; i < items.length; i++) {
		let item = items[i]
		if (x > rect.left + rect.width - 100) {
			x = rect.left + 48
			y += 10
		}
		rc.fillStyle = item.color
		rc.fillRect(x, y + 4.5 - lineWidth, lineLength, lineWidth * 2)
		x += lineLength + lineSep
		rc.fillText(item.text, x, y)
		x += rc.measureText(item.text).width + itemSep
	}
}

function stamp2x(rect, view, stamp) {
	return rect.left + ((stamp - view.startStamp) / view.duration) * rect.width
}
function value2y(rect, view, value) {
	return rect.top + rect.height - ((value - view.bottomValue) / view.height) * rect.height
}
function value2yDelta(rect, view, value) {
	return ((value - view.bottomValue) / view.height) * rect.height
}

function signed(value) {
	return (value >= 0 ? '+' : '') + value
}

export function drawLine(canvasExt, rect, view, stamps, values, color, skipZero = false) {
	let rc = canvasExt.rc
	rc.beginPath()
	let started = false
	for (let i = 0; i < stamps.length; i++) {
		let stamp = stamps[i]
		let value = values[i]
		if (skipZero && value == 0) continue
		let x = stamp2x(rect, view, stamp)
		let y = value2y(rect, view, value)
		if (started) {
			rc.lineTo(x, y)
		} else {
			rc.moveTo(x, y)
			started = true
		}
	}
	rc.lineJoin = 'bevel'
	rc.strokeStyle = color
	rc.stroke()
}
export function drawStacked(canvasExt, rect, view, stamps, values, color, accum) {
	let rc = canvasExt.rc
	rc.beginPath()
	let started = false
	let x = 0
	let y = 0
	for (let i = 0; i < stamps.length; i++) {
		let stamp = stamps[i]
		let value = values[i]
		x = stamp2x(rect, view, stamp)
		y = value2yDelta(rect, view, value)
		accum[i] += y
		if (started) {
			rc.lineTo(x, rect.top + rect.height - accum[i])
		} else {
			rc.moveTo(x, rect.top + rect.height - accum[i])
			started = true
		}
	}
	rc.lineWidth = 1
	rc.strokeStyle = 'rgba(0,0,0,0.25)'
	rc.stroke()

	rc.lineTo(x, rect.top + rect.height)
	rc.lineTo(stamp2x(rect, view, stamps[0]), rect.top + rect.height)
	rc.fillStyle = color
	rc.fill()
}

export function drawDailyBars(canvasExt, rect, view, dailyValues, posColor, negColor, textColor) {
	let rc = canvasExt.rc

	let maxLabelWidth = 1
	for (let i = 0; i < dailyValues.length; i++) {
		let width = rc.measureText(signed(dailyValues[i])).width
		if (width > maxLabelWidth) maxLabelWidth = width
	}
	let dayWidth = (rect.width / view.duration) * 24 * 3600 * 1000
	let isHorizMode = maxLabelWidth < dayWidth

	for (let [dayNum, dayDate, nextDayDate] of iterateDays(view.startStamp, view.endStamp)) {
		let value = dailyValues[dayNum]
		let x0 = stamp2x(rect, view, dayDate)
		let x1 = stamp2x(rect, view, nextDayDate)
		let y = value2y(rect, view, Math.abs(value))
		rc.fillStyle = value > 0 ? posColor : negColor
		rc.fillRect(x0, y, x1 - x0, rect.height - y)
		rc.fillStyle = textColor
		if (isHorizMode) {
			rc.textAlign = 'center'
			rc.textBaseline = 'bottom'
			rc.fillText(signed(value), (x0 + x1) / 2, y)
		} else {
			rc.textAlign = 'left'
			rc.textBaseline = 'middle'
			rc.save()
			rc.translate((x0 + x1) / 2, y - 1)
			rc.rotate(-Math.PI / 2)
			rc.fillText(signed(value), 0, 0)
			rc.restore()
		}
	}
}

export function hoverSingle({ elem, onHover, onLeave }) {
	function move(e) {
		let box = elem.getBoundingClientRect()
		onHover(e.clientX - box.left, e.clientY - box.top, e, null)
	}
	function leave(e) {
		let box = elem.getBoundingClientRect()
		onLeave(e.clientX - box.left, e.clientY - box.top, e, null)
	}

	function touchMove(e) {
		if (e.targetTouches.length > 1) return

		let box = elem.getBoundingClientRect()
		let t0 = e.targetTouches[0]

		onHover(t0.clientX - box.left, t0.clientY - box.top, e, t0)
		e.preventDefault()
	}

	elem.addEventListener('mousemove', move, true)
	elem.addEventListener('mouseleave', leave, true)
	elem.addEventListener('touchstart', touchMove, true)
	elem.addEventListener('touchmove', touchMove, true)
}
