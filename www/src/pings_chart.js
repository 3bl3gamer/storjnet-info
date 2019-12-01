import { PureComponent, bindHandlers, html, startOfMonth, endOfMonth } from './utils'

import './pings_chart.css'
import {
	CanvasExt,
	RectCenter,
	View,
	drawPingLine,
	drawMonthDays,
	forEachPingRegion,
	drawLabeledVScaleLeftLine,
} from './chart_utils'

function delayedRedraw() {
	let comp = null
	let redrawRequested = false

	function onRedraw() {
		redrawRequested = false
		comp.redraw()
	}

	return function() {
		if (redrawRequested) return
		redrawRequested = true
		comp = this
		requestAnimationFrame(onRedraw)
	}
}

function processPingsData(buf, startDate, endDate) {
	let pings = new Uint16Array(buf)

	let startStamp = Math.floor(startDate.getTime())
	let endStamp = Math.floor(endDate.getTime())

	let len = Math.floor((endStamp - startStamp) / 60 / 1000)
	let flatPings = new Uint16Array(len)
	let reductionN = 30
	let reducedPings = new Uint16Array(Math.floor(len / reductionN))

	// flatting
	for (let j = 0; j < pings.length; j += 1441 + 2) {
		let stamp = (pings[j] + (pings[j + 1] << 16)) * 1000
		let offset = Math.floor((stamp - startStamp) / 60 / 1000)
		let iFrom = offset < 0 ? -offset : 0
		let iTo = offset + 1440 > flatPings.length ? flatPings.length - offset : 1440
		for (let i = iFrom; i < iTo; i++) {
			flatPings[offset + i] = pings[j + 2 + i]
		}
	}

	// reducing
	for (let j = 0, ri = 0; j < flatPings.length; j += reductionN, ri++) {
		let pingSum = 0
		let count = 0
		for (let i = j; i < Math.min(j + reductionN, flatPings.length); i++) {
			let value = flatPings[i]
			let ping = value % 2000
			if (ping > 1) {
				pingSum += ping
				count++
			}
		}
		if (count > 0) reducedPings[ri] = pingSum / count
	}

	return {
		pings: flatPings,
		reducedPings,
		reducedPingsN: reductionN,
	}
}

class PingsChart extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.canvasExt = new CanvasExt()
		this.requestRedraw = delayedRedraw()
		let now = new Date(Date.now() - 24 * 3600 * 1000)
		this.state = {
			startDate: startOfMonth(now),
			endDate: endOfMonth(now),
			pings: null,
			reducedPings: null,
		}
	}

	loadData() {
		fetch(`/api/user_nodes/${this.props.node.id}/pings`, {
			method: 'GET',
		})
			.then(r => r.arrayBuffer())
			.then(buf => {
				let { startDate, endDate } = this.state
				this.setState(processPingsData(buf, startDate, endDate))
				this.requestRedraw()
			})
	}

	onResize() {
		this.requestRedraw()
	}

	redraw() {
		if (!this.canvasExt.created()) return
		let { canvas, rc } = this.canvasExt
		this.canvasExt.clear()
		rc.save()
		rc.scale(this.canvasExt.pixelRatio, this.canvasExt.pixelRatio)
		rc.font = '9px sans-serif'

		let { startDate, endDate } = this.state
		let rect = new RectCenter({ left: 0, right: 0, top: 1, bottom: 11 })
		rect.update(this.canvasExt.cssWidth, this.canvasExt.cssHeight)
		let view = new View({
			startStamp: startDate.getTime(),
			endStamp: endDate.getTime(),
			bottomValue: 0,
			topValue: 1000,
		})

		rc.fillStyle = '#EEE'
		rc.fillRect(0, 0, rect.width, rect.top + rect.height)

		let pings = this.state.pings
		if (pings != null) {
			rc.fillStyle = 'limegreen'
			forEachPingRegion(rect, view, pings, view.startStamp, 60 * 1000, 2, (xFrom, xTo) => {
				rc.fillRect(xFrom, 0, xTo - xFrom, 29)
			})

			drawPingLine(
				this.canvasExt,
				rect,
				view,
				this.state.reducedPings,
				view.startStamp,
				60 * 1000 * this.state.reducedPingsN,
				'rgba(0,0,0,0.4)',
			)

			rc.fillStyle = 'red'
			forEachPingRegion(rect, view, pings, view.startStamp, 60 * 1000, 1, (xFrom, xTo) => {
				rc.fillRect(xFrom - 0.25, 0, xTo - xFrom + 0.5, 29)
			})
		}

		drawMonthDays(this.canvasExt, rect, view, { hLineColor: 'rgba(0,0,0,0.1)' })

		// drawVScalesLeft(this.canvasExt, rect, view, 'black', 'rgba(0,0,0,0.12)')
		const msFunc = v => (v == 0 ? '0' : (v / 1000).toFixed(0) + 'K')
		rc.textAlign = 'left'
		rc.textBaseline = 'bottom'
		drawLabeledVScaleLeftLine(rc, rect, view, 0, 'black', null, 0, msFunc)
		rc.textBaseline = 'top'
		drawLabeledVScaleLeftLine(rc, rect, view, 1000, 'black', null, 0, msFunc)

		rc.strokeStyle = 'rgba(0,0,0,0.05)'
		rc.lineWidth = 0.5
		rc.strokeRect(0.5, 0.5, canvas.width - 1, canvas.height - 1)

		rc.restore()
	}

	componentDidMount() {
		this.canvasExt.resize()
		this.requestRedraw()
		this.loadData()
		addEventListener('resize', this.onResize)
	}
	componentWillUnmount() {
		addEventListener('resize', this.onResize)
	}

	render(props, state) {
		return html`
			<div class="pings-chart">
				<canvas ref=${this.canvasExt.setRef}></canvas>
				<div class="legend">${this.props.node.id}</div>
			</div>
		`
	}
}

export class PingsChartsList extends PureComponent {
	render({ nodes }, state) {
		return nodes.map(
			n =>
				html`
					<${PingsChart} node=${n} />
				`,
		)
	}
}
