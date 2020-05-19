import { h, render, Component } from 'preact'
import htm from 'htm'

export const html = htm.bind(h)

export function renderIfExists(comp, selector) {
	let elem = document.querySelector(selector)
	if (elem !== null) render(comp, elem)
}

export function bindHandlers(comp) {
	Object.getOwnPropertyNames(comp.constructor.prototype).forEach(name => {
		let attr = comp.__proto__[name]
		if (typeof attr == 'function' && /^on[A-Z]/.test(name)) comp[name] = attr.bind(comp)
	})
}

export function isElemIn(parent, elem) {
	while (elem !== null) {
		if (parent === elem) return true
		elem = elem.parentElement
	}
	return false
}

// vvv from preact-compat vvv

function shallowDiffers(a, b) {
	for (let i in a) if (!(i in b)) return true
	for (let i in b) if (a[i] !== b[i]) return true
	return false
}

function F() {}

export function PureComponent(props, context) {
	Component.call(this, props, context)
}
F.prototype = Component.prototype
PureComponent.prototype = new F()
PureComponent.prototype.isPureReactComponent = true
PureComponent.prototype.shouldComponentUpdate = function (props, state) {
	return shallowDiffers(this.props, props) || shallowDiffers(this.state, state)
}

// ^^^ from preact-compat ^^^

export function onError(error) {
	// eslint-disable-next-line no-console
	console.error(error)
	// alert(error)
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

export function hoverSingle({ onHover, onLeave }) {
	let elem = null

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
	function touchOuter(e) {
		if (e.targetTouches.length > 1) return
		if (isElemIn(elem, e.target)) return
		let t0 = e.targetTouches[0]
		onLeave(0, 0, e, t0)
	}

	let events = [
		['mousemove', move],
		['mouseleave', leave],
		['touchstart', touchMove],
		['touchmove', touchMove],
	]

	function mount() {
		for (let [name, handler] of events) elem.addEventListener(name, handler, true)
		window.addEventListener('touchstart', touchOuter, true)
	}
	function unmount() {
		for (let [name, handler] of events) elem.removeEventListener(name, handler, true)
		window.removeEventListener('touchstart', touchOuter, true)
	}

	return {
		setRef: function (newElem) {
			if (newElem === null) {
				unmount()
				elem = null
			} else {
				elem = newElem
				mount()
			}
		},
	}
}

export function toISODateString(date) {
	return date.toISOString().substr(0, 10)
}

export function toISODateStringInterval({ startDate, endDate }) {
	endDate = new Date(endDate)
	endDate.setUTCDate(endDate.getUTCDate() - 1)
	return { startDateStr: toISODateString(startDate), endDateStr: toISODateString(endDate) }
}

export function delayedRedraw(redrawFunc) {
	let redrawRequested = false

	function onRedraw() {
		redrawRequested = false
		redrawFunc()
	}

	return function () {
		if (redrawRequested) return
		redrawRequested = true
		requestAnimationFrame(onRedraw)
	}
}

export function LegendItem({ color, textColor = null, children }) {
	if (textColor === null) textColor = color
	return html`
		<div class="item" style="color: ${color}">
			<div class="example" style="background-color: ${textColor}"></div>
			${children}
		</div>
	`
}

export function fixNodesCount(count) {
	// Nodes are currently counted for default port only.
	// According to the data collected for old.storjnet.info, there are 15-20% more nodes on other ports.
	// UPD 2020.05.14:
	//   According to official presentation https://youtu.be/m9yudFSVjz4?t=27 there are 6000 storage nodes.
	//   At this time there was 5000 nodes on default port. So +20% seems be still correct.
	return Math.round(count * 1.2)
	// count *= 1.2
	// const k = Math.pow(10, Math.max(0, Math.floor(Math.log10(count)) - 1))
	// return Math.round(count / k) * k
}

export function zeroes(count) {
	return new Array(count).fill(0)
}
