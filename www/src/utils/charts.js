import { html } from './htm'

/** @typedef {{left:number, right:number, top:number, bottom:number, width:number, height:number}} Rect */

/** @typedef {Array<number>|Int32Array|Float32Array} NumericArray */

export class CanvasExt {
	constructor() {
		this.canvas = /**@type {HTMLCanvasElement|null}*/ (null)
		this.rc = /**@type {CanvasRenderingContext2D|null}*/ (null)
		this.pixelRatio = 1
		this.cssWidth = 0
		this.cssHeight = 0
		this.setRef = ref => {
			this.canvas = ref
			this.rc = ref === null ? null : ref.getContext('2d')
		}
	}
	_setRealSize(w, h) {
		if (this.canvas === null) return false
		if (this.canvas.width == w && this.canvas.height == h) return false
		this.canvas.width = w
		this.canvas.height = h
		return true
	}
	created() {
		return this.canvas !== null
	}
	resize() {
		if (this.canvas === null) return false
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
		if (this.canvas === null || this.rc === null) return
		this.rc.clearRect(0, 0, this.canvas.width, this.canvas.height)
	}
}

export class RectCenter {
	/** @param {{left:number, right:number, top:number, bottom:number}} params */
	constructor({ left, right, top, bottom }) {
		this.left = left
		this.right = right
		this.top = top
		this.bottom = bottom
		this.width = 0
		this.height = 0
	}

	/**
	 * @param {number} outerWidth
	 * @param {number} outerHeight
	 */
	update(outerWidth, outerHeight) {
		this.width = outerWidth - this.left - this.right
		this.height = outerHeight - this.top - this.bottom
	}
}

export class RectBottom {
	/** @param {{left:number, right:number, bottom:number, height:number}} params */
	constructor({ left, right, bottom, height }) {
		this.left = left
		this.right = right
		this.top = 0
		this.bottom = bottom
		this.width = 0
		this.height = height
	}

	/**
	 * @param {number} outerWidth
	 * @param {number} outerHeight
	 */
	update(outerWidth, outerHeight) {
		this.width = outerWidth - this.left - this.right
		this.top = outerHeight - this.bottom - this.height
	}
}

export class RectTop {
	/** @param {{left:number, right:number, top:number, height:number}} params */
	constructor({ left, right, top, height }) {
		this.left = left
		this.right = right
		this.top = top
		this.bottom = 0
		this.width = 0
		this.height = height
	}

	/**
	 * @param {number} outerWidth
	 * @param {number} outerHeight
	 */
	update(outerWidth, outerHeight) {
		this.width = outerWidth - this.left - this.right
		this.bottom = outerHeight - this.top - this.height
	}
}

export class View {
	/** @param {{startStamp:number, endStamp:number, bottomValue:number, topValue:number}} params */
	constructor({ startStamp, endStamp, bottomValue, topValue }) {
		this.startStamp = startStamp
		this.endStamp = endStamp
		this.bottomValue = bottomValue
		this.topValue = topValue
		this.duration = this.endStamp - this.startStamp
		this.height = this.topValue - this.bottomValue
	}

	/**
	 * @param {number} startStamp
	 * @param {number} endStamp
	 */
	updateStamps(startStamp, endStamp) {
		this.startStamp = startStamp
		this.endStamp = endStamp
		this.duration = this.endStamp - this.startStamp
	}

	/**
	 * @param {number} bottomValue
	 * @param {number} topValue
	 */
	updateLimits(bottomValue, topValue) {
		this.bottomValue = bottomValue
		this.topValue = topValue
		this.height = this.topValue - this.bottomValue
	}
}

/**
 * @param {NumericArray} arr
 * @param {number} [initialValue=Infinity]
 * @param {boolean} [skipZero=false]
 */
export function getArrayMinValue(arr, initialValue = Infinity, skipZero = false) {
	let val = initialValue
	for (let i = 0; i < arr.length; i++) if (val > arr[i] && (!skipZero || arr[i] !== 0)) val = arr[i]
	return val
}

/**
 * @param {NumericArray} arr
 * @param {number} [initialValue=Infinity]
 */
export function getArrayMaxValue(arr, initialValue = -Infinity) {
	let val = initialValue
	for (let i = 0; i < arr.length; i++) if (val < arr[i]) val = arr[i]
	return val
}

/**
 * @param {number} bottomVal
 * @param {number} topVal
 * @param {number} [sigDigits=2]
 * @returns {[number, number]}
 */
export function roundRange(bottomVal, topVal, sigDigits = 2) {
	let k = Math.pow(10, Math.round(0.2 + Math.log10(topVal - bottomVal) - (sigDigits - 1)))
	return [Math.floor(bottomVal / k) * k, Math.ceil(topVal / k) * k]
}

/**
 * @param {Rect} rect
 * @param {View} view
 * @param {number|Date} stamp
 */
export function stamp2x(rect, view, stamp) {
	return rect.left + ((+stamp - view.startStamp) / view.duration) * rect.width
}
/**
 * @param {Rect} rect
 * @param {View} view
 * @param {number} value
 */
export function value2y(rect, view, value) {
	return rect.top + rect.height - ((value - view.bottomValue) / view.height) * rect.height
}
/**
 * @param {Rect} rect
 * @param {View} view
 * @param {number} value
 */
export function value2yLog(rect, view, value) {
	let maxLogVal = Math.log(view.height)
	value = ((Math.log(1 + Math.abs(value)) * Math.sign(value)) / maxLogVal) * view.height
	return rect.top + rect.height - ((value - view.bottomValue) / view.height) * rect.height
}
/** @param {number} value */
export function signed(value) {
	return (value >= 0 ? '+' : '') + value
}

export const PING_UNDEF = 0
export const PING_ERR = 1
export const PING_OK = 2
/** @typedef {typeof PING_OK | typeof PING_ERR | typeof PING_UNDEF} PingKind */

/**
 * @param {CanvasRenderingContext2D} rc
 * @param {Rect} rect
 * @param {View} view
 * @param {Uint16Array} pings
 * @param {number} startStamp
 * @param {number} itemWidth
 * @param {string} color
 */
export function drawPingLine(rc, rect, view, pings, startStamp, itemWidth, color) {
	let iFrom = Math.max(0, Math.floor((view.startStamp - startStamp) / itemWidth))
	let iTo = Math.min(pings.length, Math.ceil((view.endStamp - startStamp) / itemWidth))
	rc.beginPath()
	let started = false
	for (let i = iFrom; i < iTo; i++) {
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

/**
 * @param {Rect} rect
 * @param {View} view
 * @param {Uint16Array} pings
 * @param {number} startStamp
 * @param {number} itemWidth
 * @param {PingKind} needKind
 * @param {(prevX:number, x:number) => void} func
 */
export function forEachPingRegion(rect, view, pings, startStamp, itemWidth, needKind, func) {
	let prevKind = null
	let prevKindX = 0
	let lastX = 0
	let iFrom = Math.max(0, Math.floor((view.startStamp - startStamp) / itemWidth))
	let iTo = Math.min(pings.length, Math.ceil((view.endStamp - startStamp) / itemWidth))
	for (let i = iFrom; i < iTo; i++) {
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
/**
 * @param {CanvasRenderingContext2D} rc
 * @param {Rect} rect
 * @param {View} view
 * @param {Uint16Array} pings
 * @param {number} startStamp
 * @param {number} itemWidth
 * @param {PingKind} needKind
 * @param {string} color
 * @param {number} ext - extend region to left and right (in pixels)
 */
export function drawPingRegions(rc, rect, view, pings, startStamp, itemWidth, needKind, color, ext) {
	rc.fillStyle = color
	forEachPingRegion(rect, view, pings, startStamp, itemWidth, needKind, (xFrom, xTo) => {
		rc.fillRect(xFrom - ext / 2, rect.top, xTo - xFrom + ext, rect.height)
	})
}

/**
 * @param {CanvasRenderingContext2D} rc
 * @param {Rect} rect
 * @param {View} view
 * @param {NumericArray} values
 * @param {number} startStamp
 * @param {number} itemWidth
 * @param {string} color
 * @param {boolean} [skipZero=false]
 * @param {boolean} [joinZero=false]
 * @param {(rect:Rect, view:View, value:number) => number} [yFunc=value2y]
 */
export function drawLineStepped(
	rc,
	rect,
	view,
	values,
	startStamp,
	itemWidth,
	color,
	skipZero = false,
	joinZero = false,
	yFunc = value2y,
) {
	rc.beginPath()
	let started = false
	let prevX = 0,
		prevY = 0
	for (let i = 0; i < values.length; i++) {
		let stamp = startStamp + i * itemWidth
		let value = values[i]
		let x = stamp2x(rect, view, stamp)
		let y = yFunc(rect, view, value)
		if (skipZero && value == 0) {
			if (started) {
				if (joinZero) rc.lineTo(x, y)
				started = false
			}
		} else {
			if (started) {
				rc.lineTo(x, y)
			} else {
				if (i == 0) {
					rc.moveTo(x, y)
				} else if (joinZero) {
					rc.moveTo(prevX, prevY)
					rc.lineTo(x, y)
				} else {
					rc.moveTo(x, y)
				}
				started = true
			}
		}
		prevX = x
		prevY = y
	}
	rc.lineJoin = 'bevel'
	rc.strokeStyle = color
	rc.stroke()
}

/**
 * @param {CanvasRenderingContext2D} rc
 * @param {Rect} rect
 * @param {View} view
 * @param {NumericArray} comeValues
 * @param {NumericArray} leftValues
 * @param {string} comeColor
 * @param {string} leftColor
 * @param {string} textColor
 */
export function drawDailyComeLeftBars(
	rc,
	rect,
	view,
	comeValues,
	leftValues,
	comeColor,
	leftColor,
	textColor,
) {
	let maxLabelWidth = 1
	for (let i = 0; i < comeValues.length; i++) {
		const width = Math.max(
			rc.measureText(signed(comeValues[i])).width,
			rc.measureText(signed(-leftValues[i])).width,
		)
		if (width > maxLabelWidth) maxLabelWidth = width
	}
	let dayWidth = (rect.width / view.duration) * 24 * 3600 * 1000
	let isHorizMode = maxLabelWidth < dayWidth

	for (let [dayNum, dayDate, nextDayDate] of iterateDays(view.startStamp, view.endStamp, 1, true)) {
		const come = comeValues[dayNum]
		const left = leftValues[dayNum]
		const x0 = stamp2x(rect, view, dayDate)
		const x1 = stamp2x(rect, view, nextDayDate)
		const margin = 0.05
		const shift = 0.8
		const dx = x1 - x0
		const comeY = value2y(rect, view, Math.abs(come))
		const leftY = value2y(rect, view, Math.abs(left))

		if (come > left) {
			rc.fillStyle = comeColor
			rc.fillRect(x0 + (dx * margin) / 2, comeY, dx * (shift - margin), rect.height - comeY)
		}
		rc.fillStyle = leftColor
		rc.fillRect(x0 + dx * (1 - shift + margin / 2), leftY, dx * (shift - margin), rect.height - leftY)
		if (come <= left) {
			rc.fillStyle = comeColor
			rc.fillRect(x0 + (dx * margin) / 2, comeY, dx * (shift - margin), rect.height - comeY)
		}

		const textY = Math.min(comeY, leftY)
		rc.fillStyle = textColor
		if (isHorizMode) {
			rc.textAlign = 'center'
			rc.textBaseline = 'bottom'
			rc.fillText(signed(-left), (x0 + x1) / 2, textY)
			rc.fillText(signed(come), (x0 + x1) / 2, textY - 12)
		} else {
			rc.textAlign = 'left'
			rc.textBaseline = 'middle'
			rc.save()
			rc.translate((x0 + x1) / 2, textY - 1)
			rc.rotate(-Math.PI / 2)
			rc.fillText(signed(come - left), 0, 0)
			rc.restore()
		}
	}
}

/**
 * @param {Date|number} startDate
 * @param {Date|number} endDate
 * @param {number} [step=1]
 * @param {boolean} [utc=false]
 * @returns {Generator<[number, Date, Date], void, void>}
 */
export function* iterateDays(startDate, endDate, step = 1, utc = false) {
	if (step < 1) step = 1
	startDate = new Date(startDate)
	startDate[utc ? 'setUTCHours' : 'setHours'](0, 0, 0, 0)

	let dayDate = new Date(startDate)
	for (let dayNum = 0; ; dayNum += step) {
		let nextDayDate = new Date(startDate)
		if (utc) {
			nextDayDate.setUTCDate(nextDayDate.getUTCDate() + dayNum + 1)
		} else {
			nextDayDate.setDate(nextDayDate.getDate() + dayNum + 1)
		}

		yield [dayNum, dayDate, nextDayDate]

		if (nextDayDate >= endDate) break
		dayDate = nextDayDate
	}
}

/**
 * @param {CanvasExt} canvasExt
 * @param {Rect} rect
 * @param {View} view
 * @param {{textColor?:string, vLinesColor?:string|null, hLineColor?:string, textYShift?:number}} [params={}]
 */
export function drawMonthDays(canvasExt, rect, view, params = {}) {
	let { textColor = 'black', vLinesColor = null, hLineColor = '#555', textYShift = 2 } = params

	let rc = canvasExt.rc
	if (rc === null) return
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
			y + textYShift,
			rect.height - 2,
			vLinesColor,
		)
		let x = stamp2x(rect, view, dayDate)
		if (vLinesColor !== null) {
			rc.fillStyle = vLinesColor
			rc.fillRect(x, y, 1, -rect.height)
		}
		rc.fillStyle = textColor
		rc.fillText(dayDate.getDate() + '', x, y + textYShift)
	}

	if (hLineColor !== null) {
		rc.fillStyle = hLineColor
		rc.fillRect(rect.left, rect.top + rect.height - 0.5, rect.width, 1)
	}
}
/**
 * @param {CanvasExt} canvasExt
 * @param {number} dateDrawFrom
 * @param {number} dateDrawTo
 * @param {Date} curDayDate
 * @param {Date} nextDayDate
 * @param {number} y
 * @param {number} markHeight
 * @param {string|null} markColor
 */
function drawHours(canvasExt, dateDrawFrom, dateDrawTo, curDayDate, nextDayDate, y, markHeight, markColor) {
	let rc = canvasExt.rc
	if (rc === null) return

	let labelWidth = rc.measureText('12:00').width * 1.5
	let dayWidth = (canvasExt.cssWidth * (+nextDayDate - +curDayDate)) / (dateDrawTo - dateDrawFrom)
	let maxLabels = dayWidth / labelWidth
	let step = [1, 2, 3, 4, 6, 12].find(x => x > 24 / maxLabels)
	if (step == null) return

	rc.strokeStyle = 'rgba(255,255,255,0.3)'
	rc.lineWidth = 2.5
	for (let hourNum = step; ; hourNum += step) {
		let curDate = new Date(curDayDate)
		curDate.setHours(curDate.getHours() + hourNum)
		if (curDate >= nextDayDate) break
		let x = ((curDate.getTime() - dateDrawFrom) / (dateDrawTo - dateDrawFrom)) * canvasExt.cssWidth
		if (markColor !== null) {
			rc.fillStyle = markColor
			rc.fillRect(x, y - 2, 1, -markHeight)
		}
		rc.strokeText(curDate.getHours() + ':00', x, y)
		rc.fillStyle = '#777'
		rc.fillText(curDate.getHours() + ':00', x, y)
	}
}

/**
 * @param {number} bottomValue
 * @param {number} topValue
 * @param {number} roundN
 */
function roundLabelValues(bottomValue, topValue, roundN) {
	let k = Math.pow(10, roundN)

	bottomValue = Math.ceil(bottomValue / k) * k
	topValue = Math.floor(topValue / k) * k
	let midValue = (topValue + bottomValue) / 2
	// let midValue = Math.sqrt(topValue + bottomValue)

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
/**
 * @param {CanvasRenderingContext2D} rc
 * @param {Rect} rect
 * @param {View} view
 * @param {number} value
 * @param {string} textColor
 * @param {string|null} lineColor
 * @param {number} roundN
 * @param {((value:number) => string)|null} [textFunc=null]
 * @param {(rect:Rect, view:View, value:number) => number} [yFunc=value2y]
 */
export function drawLabeledVScaleLeftLine(
	rc,
	rect,
	view,
	value,
	textColor,
	lineColor,
	roundN,
	textFunc = null,
	yFunc = value2y,
) {
	let text = textFunc === null ? value.toFixed(Math.max(0, -roundN)) : textFunc(value)
	let lineY = yFunc(rect, view, value)
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
/**
 * @param {CanvasRenderingContext2D} rc
 * @param {Rect} rect
 * @param {View} view
 * @param {string} textColor
 * @param {string} lineColor
 * @param {((value:number) => string)|null} [textFunc=null]
 */
export function drawVScalesLeft(rc, rect, view, textColor, lineColor, textFunc = null) {
	let roundN = Math.floor(Math.log10(view.topValue - view.bottomValue) - 1)
	// let topOffset = (view.height / rect.height) * 11 //font height
	let topOffset = 0 //Math.exp(Math.log(view.height) - (Math.log(view.height) / rect.height) * 11) //font height
	let values = roundLabelValues(view.bottomValue, view.topValue - topOffset, roundN)
	rc.textAlign = 'left'
	rc.textBaseline = 'middle'
	for (let i = 0; i < values.length; i++)
		drawLabeledVScaleLeftLine(rc, rect, view, values[i], textColor, lineColor, roundN, textFunc, value2y)
}

/**
 * @param {CanvasRenderingContext2D} rc
 * @param {number} x
 * @param {number} y
 * @param {number} h
 * @param {number} r
 */
export function roundedRectLeft(rc, x, y, h, r) {
	rc.lineTo(x + r, y + h)
	rc.arcTo(x, y + h, x, y + h - r, r)
	rc.lineTo(x, y + y + r)
	rc.arcTo(x, y, x + r, y, r)
}
/**
 * @param {CanvasRenderingContext2D} rc
 * @param {number} x
 * @param {number} y
 * @param {number} h
 * @param {number} r
 */
export function roundedRectRight(rc, x, y, h, r) {
	rc.lineTo(x - r, y)
	rc.arcTo(x, y, x, y + r, r)
	rc.lineTo(x, y + h - r)
	rc.arcTo(x, y + h, x - r, y + h, r)
}
/**
 * @param {CanvasRenderingContext2D} rc
 * @param {number} x
 * @param {number} y
 * @param {number} h
 * @param {number} r
 */
export function roundedRect(rc, x, y, w, h, r) {
	rc.moveTo(x + w - r, y + h)
	roundedRectLeft(rc, x, y, h, r)
	roundedRectRight(rc, x + w, y, h, r)
}

/**
 * От нормального <${LegendItem}>children</${LegendItem}> ломается автоформатирование,
 * в качестве фикса — свойство text.
 * @param {{color:string, textColor:string|null, text:string|null, children:import('preact').JSX.Element}} params
 */
export function LegendItem({ color, textColor = null, text = null, children }) {
	if (textColor === null) textColor = color
	return html`
		<div class="item" style="color: ${color}">
			<div class="example" style="background-color: ${textColor}"></div>
			${text || children}
		</div>
	`
}
