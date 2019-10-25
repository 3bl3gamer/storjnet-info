import './main.css'
import { h, render } from 'preact'
import htm from 'htm'

const html = htm.bind(h)

const app1 = html`
	<div>Hello World 1</div>
`
const app2 = html`
	<div>Hello World 2</div>
`
render(app1, document.querySelector('#e1'))
render(app2, document.querySelector('#e2'))
console.log(app1, app2)
