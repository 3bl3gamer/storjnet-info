import { h, render } from 'preact'
import { useLayoutEffect } from 'preact/hooks'

export function renderIfExists(Comp, selector) {
	let elem = document.querySelector(selector)
	if (elem !== null) render(h(Comp, null), elem)
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

/** @param {string} elemId */
export function getJSONContent(elemId) {
	let elem = document.getElementById(elemId)
	if (elem === null) throw new Error(`elem #${elemId} not found`)
	return JSON.parse(elem.textContent + '')
}

/**
 * @param {() => unknown} onResize
 * @param {unknown[]} args
 */
export function useResizeEffect(onResize, args) {
	useLayoutEffect(() => {
		addEventListener('resize', onResize)
		return () => {
			removeEventListener('resize', onResize)
		}
	}, args)
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
