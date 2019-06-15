export class CanvasExt {
	constructor(canvas) {
		this.canvas = canvas
		this.rc = this.canvas.getContext('2d')
		this.pixelRatio = 1
		this.cssWidth = this.canvas.width
		this.cssHeight = this.canvas.height
	}
	static createIn(wrap) {
		let canvas = document.createElement('canvas')
		let ce = new CanvasExt(canvas)
		wrap.appendChild(canvas)
		return ce
	}
	_setRealSize(w, h) {
		if (this.canvas.width == w && this.canvas.height == h) return false
		this.canvas.width = w
		this.canvas.height = h
		return true
	}
	resize() {
		let { width: cssWidth, height: cssHeight } = this.canvas.getBoundingClientRect()
		this.pixelRatio = window.devicePixelRatio
		this.cssWidth = cssWidth
		this.cssHeight = cssHeight
		let width = Math.floor(this.cssWidth * this.pixelRatio + 0.249)
		let height = Math.floor(this.cssHeight * this.pixelRatio + 0.249)
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

export function drawMonthDays(canvasExt, dateDrawFrom, dateDrawTo, y) {
	let dateFrom = new Date(dateDrawFrom)
	dateFrom.setHours(0, 0, 0, 0)

	let rc = canvasExt.rc
	rc.fillStyle = 'black'
	rc.textAlign = 'center'
	rc.textBaseline = 'top'

	for (let dayNum = 0; ; dayNum++) {
		let dayDate = new Date(dateFrom)
		dayDate.setDate(dayDate.getDate() + dayNum)
		if (dayDate.getTime() >= dateDrawTo) break

		let x =
			((dayDate - dateDrawFrom) / (dateDrawTo - dateDrawFrom)) * canvasExt.cssWidth
		rc.fillRect(x, y - 3, 1, 3)
		rc.fillText(dayDate.getDate(), x, y + 2)
	}
}
