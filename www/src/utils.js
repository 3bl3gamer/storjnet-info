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

// from preact-compat

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
PureComponent.prototype.shouldComponentUpdate = function(props, state) {
	return shallowDiffers(this.props, props) || shallowDiffers(this.state, state)
}
