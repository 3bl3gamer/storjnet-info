import { Component } from 'preact'

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
