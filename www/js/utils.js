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

export function drawMonthDays(canvasExt, dateDrawFrom, dateDrawTo, y, markHeight) {
	let dateFrom = new Date(dateDrawFrom)
	dateFrom.setHours(0, 0, 0, 0)

	let rc = canvasExt.rc
	rc.fillStyle = 'black'
	rc.textAlign = 'center'
	rc.textBaseline = 'top'

	let labelWidth = rc.measureText('30').width * 1.5
	let approxNumDays = (dateDrawTo - dateDrawFrom) / (24 * 3600 * 1000)
	let maxLabels = canvasExt.cssWidth / labelWidth
	let step = Math.ceil(approxNumDays / maxLabels)

	let prevDayDate = null
	for (let dayNum = 0; ; dayNum += step) {
		let dayDate = new Date(dateFrom)
		dayDate.setDate(dayDate.getDate() + dayNum)

		if (prevDayDate !== null)
			drawHours(
				canvasExt,
				dateDrawFrom,
				dateDrawTo,
				prevDayDate,
				dayDate,
				y,
				markHeight - 2,
			)
		if (dayDate.getTime() >= dateDrawTo) break

		rc.fillStyle = 'black'
		let x =
			((dayDate - dateDrawFrom) / (dateDrawTo - dateDrawFrom)) * canvasExt.cssWidth
		rc.fillRect(x, y, 1, -markHeight)
		rc.fillText(dayDate.getDate(), x, y + 2)

		prevDayDate = dayDate
	}
}
function drawHours(
	canvasExt,
	dateDrawFrom,
	dateDrawTo,
	curDayDate,
	nextDayDate,
	y,
	markHeight,
) {
	let rc = canvasExt.rc

	let labelWidth = rc.measureText('12:00').width * 1.5
	let dayWidth =
		(canvasExt.cssWidth * (nextDayDate - curDayDate)) / (dateDrawTo - dateDrawFrom)
	let maxLabels = dayWidth / labelWidth
	let step = [1, 2, 3, 4, 6, 12].find(x => x > 24 / maxLabels)
	if (step == null) return

	rc.strokeStyle = 'rgba(255,255,255,0.3)'
	rc.lineWidth = 2.5
	for (let hourNum = step; ; hourNum += step) {
		let curDate = new Date(curDayDate)
		curDate.setHours(curDate.getHours() + hourNum)
		if (curDate >= nextDayDate) break
		let x =
            ((curDate - dateDrawFrom) / (dateDrawTo - dateDrawFrom)) * canvasExt.cssWidth
        rc.fillStyle = 'black'
		rc.fillRect(x, y, 1, -markHeight)
        rc.strokeText(curDate.getHours() + ':00', x, y + 2)
        rc.fillStyle = '#777'
		rc.fillText(curDate.getHours() + ':00', x, y + 2)
	}
}

export function hoverSingle({ elem, onHover, onLeave }) {
	function move(e) {
		let box = elem.getBoundingClientRect()
		onHover(e.clientX - box.left, e.clientY - box.top, e)
	}
	function leave(e) {
		let box = elem.getBoundingClientRect()
		onLeave(e.clientX - box.left, e.clientY - box.top, e)
	}

	function touchMove(e) {
		if (e.targetTouches.length > 1) return

		let box = elem.getBoundingClientRect()
		let t0 = e.targetTouches[0]

		onHover(t0.clientX - box.left, t0.clientY - box.top, e)
	}

	elem.addEventListener('mousemove', move, true)
	elem.addEventListener('mouseleave', leave, true)
	elem.addEventListener('touchmove', touchMove, true)
}
