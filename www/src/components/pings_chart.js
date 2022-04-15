import { apiReq } from 'src/api'
import { L, lang } from 'src/i18n'
import { onError } from 'src/errors'
import {
	CanvasExt,
	RectCenter,
	View,
	drawPingLine,
	drawMonthDays,
	drawLabeledVScaleLeftLine,
	RectTop,
	roundedRect,
	drawPingRegions,
	PING_ERR,
	PING_OK,
} from 'src/utils/charts'
import { shortNodeID, sortNodes } from 'src/utils/nodes'
import { DAY_DURATION, intervalIsDefault, toISODateStringInterval, watchHashInterval } from 'src/utils/time'
import { memo, PureComponent } from 'src/utils/preact_compat'
import { bindHandlers, delayedRedraw, getJSONContent, hoverSingle } from 'src/utils/elems'
import { html } from 'src/utils/htm'

import './pings_chart.css'
import { Fragment } from 'preact'

/** @typedef {{id:string, address:string}} PingNode */

/**
 * Data mode, short of full.
 *
 * In both modes one value correspondes to one minute.
 *
 * In short mode each value is 1-byte:
 *   ping_ms = (raw_byte_value+0.5) * 2000 / 256
 *   seconds_from_start_of_minute = 30
 *   raw_byte_value == 0 - no data
 *   raw_byte_value == 1 - error/timeout
 *
 * In full mode each each value is 2-bytes:
 *   ping_ms = raw_byte_value % 2000
 *   seconds_from_start_of_minute = floor(raw_byte_value / 2000) * 4
 *   ping_ms == 0 - no data (here also raw_byte_value == 0)
 *   ping_ms == 1 - error/timeout
 */
const PING_DATA_SHORT_MODE = true

/**
 * @param {ArrayBuffer} buf
 * @param {Date} startDate
 * @param {Date} endDate
 */
function processPingsData(buf, startDate, endDate) {
	let pings = PING_DATA_SHORT_MODE ? new Uint8Array(buf) : new Uint16Array(buf)

	let startStamp = Math.floor(startDate.getTime())
	let endStamp = Math.floor(endDate.getTime() + DAY_DURATION)

	let len = Math.floor((endStamp - startStamp) / 60 / 1000)
	let flatPings = new Uint16Array(len)
	let reductionN = 30
	let reducedPings = new Uint16Array(Math.floor(len / reductionN))

	// flatting
	for (let j = 0; j < pings.length; j += 1440 + (PING_DATA_SHORT_MODE ? 4 : 2)) {
		let stamp = PING_DATA_SHORT_MODE
			? (pings[j] + (pings[j + 1] << 8) + (pings[j + 2] << 16) + (pings[j + 3] << 24)) * 1000
			: (pings[j] + (pings[j + 1] << 16)) * 1000
		let offset = Math.floor((stamp - startStamp) / 60 / 1000)
		let iFrom = offset < 0 ? -offset : 0
		let iTo = offset + 1440 > flatPings.length ? flatPings.length - offset : 1440
		for (let i = iFrom; i < iTo; i++) {
			if (PING_DATA_SHORT_MODE) {
				let val = pings[j + 4 + i]
				flatPings[offset + i] = val <= 1 ? val : 7 * 2000 + ((pings[j + 4 + i] + 0.5) * 2000) / 256
			} else {
				flatPings[offset + i] = pings[j + 2 + i]
			}
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

/** @param {string} address */
function cutOffDefaultSatPort(address) {
	let m = address.match(/^(.*?)(:7777)?$/)
	return m ? m[1] : address
}

/**
 * @class
 * @typedef PC_Props
 * @prop {PingNode} node
 * @prop {'my'|'sat'} group
 * @prop {boolean} isPending
 * @typedef PC_State
 * @prop {Date} startDate
 * @prop {Date} endDate
 * @prop {Uint16Array|null} pings
 * @prop {Uint16Array|null} reducedPings
 * @prop {number} reducedPingsN
 * @prop {{isShown:boolean, cusorX:number, boxX:number, boxWidth:number, pos:number, isTouch:boolean}} zoom
 * @extends {PureComponent<PC_Props, PC_State>}
 */
class PingsChart extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.canvasExt = new CanvasExt()
		this.canvasZoomExt = new CanvasExt()

		this.requestRedraw = delayedRedraw(this.onRedraw)
		this.requestZoomRedraw = delayedRedraw(this.onZoomRedraw)

		this.hoverCtl = hoverSingle({ onHover: this.onHover, onLeave: this.onLeave })

		this.rect = new RectCenter({ left: 0, right: 0, top: 0, bottom: 11 })
		this.view = new View({
			startStamp: 0,
			endStamp: 0,
			bottomValue: 0,
			topValue: 1000,
		})

		this.zoomRect = new RectTop({ left: 0, right: 0, height: 32, top: 0 })
		this.zoomLabelsRect = new RectTop({ left: 0, right: 0, height: 5, top: 28 })
		this.zoomView = new View({
			startStamp: 0,
			endStamp: 0,
			bottomValue: 0,
			topValue: 2000,
		})

		let watch = watchHashInterval((startDate, endDate) => {
			let onSet = () => this.loadData()
			this.setState({ ...this.state, startDate, endDate, pings: null, reducedPings: null }, onSet)
		})
		this.stopWatchingHashInterval = watch.off

		/** @type {PC_State} */
		this.state = {
			startDate: watch.startDate,
			endDate: watch.endDate,
			pings: null,
			reducedPings: null,
			reducedPingsN: 0,
			zoom: { isShown: false, cusorX: 0, boxX: 0, boxWidth: 0, pos: 0, isTouch: false },
		}
	}

	loadData() {
		if (this.props.isPending) return
		let { startDateStr: start, endDateStr: end } = toISODateStringInterval(this.state)
		apiReq('GET', `/api/user_nodes/${this.props.group}/${this.props.node.id}/pings`, {
			data: { start_date: start, end_date: end },
		})
			.then(r => r.arrayBuffer())
			.then(buf => {
				let { startDate, endDate } = this.state
				this.setState(processPingsData(buf, startDate, endDate))
				this.requestRedraw()
			})
			.catch(onError)
	}

	onResize() {
		this.requestRedraw()
		this.setState({ zoom: { ...this.state.zoom, isShown: false } })
	}
	onHover(x, y, e, touch) {
		if (this.canvasExt.cssWidth === 0) return
		let isTouch = !!touch
		let boxWidth = Math.min(512, this.canvasExt.cssWidth)
		let pixRatio = window.devicePixelRatio
		let boxXMax = this.canvasExt.cssWidth - boxWidth
		let boxX = Math.max(0, Math.min(x - boxWidth / 2, boxXMax))
		boxX = Math.round(boxX * pixRatio) / pixRatio
		let pos = x / this.canvasExt.cssWidth
		this.setState({ zoom: { isShown: true, cusorX: x, boxX, boxWidth, pos, isTouch } })
		this.requestZoomRedraw()
	}
	onLeave() {
		this.setState({ zoom: { ...this.state.zoom, isShown: false } })
	}

	onRedraw() {
		let { canvasExt, rect, view } = this
		let { pings, reducedPings, reducedPingsN, startDate, endDate } = this.state
		let { rc } = canvasExt

		if (!canvasExt.created() || rc === null) return
		canvasExt.resize()

		canvasExt.clear()
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)
		rc.font = '9px sans-serif'

		rect.update(canvasExt.cssWidth, canvasExt.cssHeight)
		view.updateStamps(startDate.getTime(), endDate.getTime() + DAY_DURATION)

		rc.fillStyle = '#EEE'
		rc.fillRect(0, 0, rect.width, rect.top + rect.height)

		if (pings !== null && reducedPings !== null) {
			drawPingRegions(rc, rect, view, pings, +startDate, 60 * 1000, PING_OK, 'limegreen', 0.5)

			drawPingLine(
				rc,
				rect,
				view,
				reducedPings,
				+startDate,
				60 * 1000 * reducedPingsN,
				'rgba(0,0,0,0.4)',
			)

			drawPingRegions(rc, rect, view, pings, +startDate, 60 * 1000, PING_ERR, 'red', 0.5)
		}

		drawMonthDays(canvasExt, rect, view, { hLineColor: 'rgba(0,0,0,0.1)' })

		const msFunc = v => (v === 0 ? '0' : (v / 1000).toFixed(0) + 'K')
		rc.textAlign = 'left'
		rc.textBaseline = 'bottom'
		drawLabeledVScaleLeftLine(rc, rect, view, 0, 'black', null, 0, msFunc)
		rc.textBaseline = 'top'
		drawLabeledVScaleLeftLine(rc, rect, view, 1000, 'black', null, 0, msFunc)

		rc.strokeStyle = 'rgba(0,0,0,0.05)'
		rc.lineWidth = 0.5
		rc.strokeRect(0.5, 0.5, rect.width - 1, rect.top + rect.height - 0.5)

		rc.restore()
	}

	onZoomRedraw() {
		let { canvasZoomExt: canvasExt, zoomView: view } = this
		let { rect: mainRect, zoomRect: rect, zoomLabelsRect: labelsRect } = this
		let { zoom, pings, startDate, endDate } = this.state
		let { rc } = canvasExt

		if (!canvasExt.created() || rc === null) return
		canvasExt.resize()

		rect.update(canvasExt.cssWidth, canvasExt.cssHeight)
		labelsRect.update(canvasExt.cssWidth, canvasExt.cssHeight)

		let timeW = 24 * 3600 * 1000
		let stamp = +startDate + zoom.pos * (+endDate + DAY_DURATION - startDate.getTime())
		view.updateStamps(stamp - timeW, stamp + timeW)

		canvasExt.clear()
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)

		rc.fillStyle = 'black'
		rc.fillRect(zoom.cusorX - zoom.boxX - 0.5, canvasExt.cssHeight - mainRect.bottom, 1, -mainRect.height)

		rc.fillStyle = 'rgba(255,255,255,0.7)'
		rc.fillRect(0, 4, canvasExt.cssWidth, rect.height + 12 - 4)

		rc.save()
		rc.beginPath()
		roundedRect(rc, 0.5, 0.5, rect.width - 1, rect.height, 2.5)
		rc.clip()

		rc.fillStyle = '#EEE'
		rc.fillRect(0, 0, rect.width, rect.top + rect.height)

		if (pings !== null) {
			drawPingRegions(rc, rect, view, pings, +startDate, 60 * 1000, 2, 'limegreen', 0.5)
			drawPingLine(rc, rect, view, pings, +startDate, 60 * 1000, 'rgba(0,0,0,0.5)')
			drawPingRegions(rc, rect, view, pings, +startDate, 60 * 1000, 1, 'red', 0.5)
		}

		rc.restore()

		drawMonthDays(canvasExt, labelsRect, view, {
			hLineColor: 'rgba(0,0,0,0.1)',
			vLinesColor: 'black',
		})

		rc.beginPath()
		roundedRect(rc, 0.5, 0.5, rect.width - 1, rect.height, 2.5)
		rc.lineWidth = 1
		rc.strokeStyle = 'rgba(0,0,0,0.8)'
		rc.stroke()

		rc.restore()
	}

	componentDidMount() {
		this.requestRedraw()
		this.loadData()
		addEventListener('resize', this.onResize)
	}
	componentWillUnmount() {
		addEventListener('resize', this.onResize)
		this.stopWatchingHashInterval()
	}

	/**
	 * @param {PC_Props} props
	 * @param {PC_State} state
	 */
	render({ node, group }, { zoom }) {
		let zoomElem =
			zoom.isShown &&
			html`
				<canvas
					class="zoom-canvas ${zoom.isTouch ? 'touch' : ''}"
					ref=${this.canvasZoomExt.setRef}
					style="width: ${zoom.boxWidth}px; transform: translateX(${zoom.boxX}px)"
				></canvas>
			`
		let legend = group === 'sat' ? cutOffDefaultSatPort(node.address) : shortNodeID(node.id)
		return html`
			<div class="chart pings-chart" ref=${this.hoverCtl.setRef}>
				<canvas class="main-canvas" ref=${this.canvasExt.setRef}></canvas>
				<div class="legend">${legend}</div>
				${zoomElem}
			</div>
		`
	}
}

export class PingsChartsList extends PureComponent {
	render({ nodes, group }, state) {
		return nodes.map(n => html` <${PingsChart} group=${group} node=${n} isPending=${false} /> `)
	}
}

/**
 * @class
 * @typedef SPCL_Props
 * @prop {PingNode[]} defaultSatNodes
 * @typedef SPCL_State
 * @prop {Date} startDate
 * @prop {Date} endDate
 * @prop {PingNode[]} currentSatNodes
 * @prop {boolean} isLoaded
 * @extends {PureComponent<SPCL_Props, SPCL_State>}
 */
export class SatsPingsChartsList extends PureComponent {
	constructor({ defaultSatNodes }) {
		super()

		let watch = watchHashInterval((startDate, endDate) => {
			let onSet = () => this.checkInterval()
			this.setState({ ...this.state, startDate, endDate }, onSet)
		})
		this.stopWatchingHashInterval = watch.off

		/** @type {SPCL_State} */
		this.state = {
			startDate: watch.startDate,
			endDate: watch.endDate,
			currentSatNodes: defaultSatNodes,
			isLoaded: intervalIsDefault(),
		}
	}

	checkInterval() {
		if (intervalIsDefault()) {
			this.setState({ currentSatNodes: this.props.defaultSatNodes, isLoaded: true })
		} else {
			this.loadSatNodes()
		}
	}
	loadSatNodes() {
		this.setState({ ...this.state, isLoaded: false })
		let { startDateStr: start, endDateStr: end } = toISODateStringInterval(this.state)
		apiReq('GET', `/api/sat_nodes`, {
			data: { start_date: start, end_date: end },
		})
			.then(sats => {
				this.setState({ ...this.state, currentSatNodes: sortNodes(sats), isLoaded: true })
			})
			.catch(onError)
	}

	componentDidMount() {
		this.checkInterval()
	}
	componentWillUnmount() {
		this.stopWatchingHashInterval()
	}

	/**
	 * @param {SPCL_Props} props
	 * @param {SPCL_State} state
	 */
	render(props, { currentSatNodes, isLoaded, startDate, endDate }) {
		const noteDate = new Date('2022-04-14T12:00:00Z')
		const note =
			startDate < noteDate && endDate > noteDate
				? lang === 'ru'
					? 'сервер переехал из Парижа в Санкт-Петербург, пинги изменились.'
					: 'the server was moved from Paris to St. Petersburg, ping times have changed.'
				: null

		return html`<${Fragment}>
			${currentSatNodes.map(
				(n, i) =>
					html`
						<!-- using key to trigger PingsChart remount (and load) on isLoaded change, TODO -->
						<${PingsChart}
							key=${i + '|' + isLoaded}
							group="sat"
							node=${n}
							isPending=${!isLoaded}
						/>
					`,
			)}
			${note && html`<p class="warn small"><b>${noteDate.toISOString().slice(0, 10)}:</b> ${note}</p>`}
		</${Fragment}>`
	}
}

/** @type {PingNode[]} */
let defaultSatNodes = []
try {
	defaultSatNodes = sortNodes(getJSONContent('sat_nodes_data'))
} catch (ex) {
	// ¯\_(ツ)_/¯
}
export const SatsPingsCharts = memo(function SatsPingsCharts() {
	return html`
		<h2>${L('Satellites', 'ru', 'Сателлиты')}</h2>
		<${SatsPingsChartsList} defaultSatNodes=${defaultSatNodes} />
		<p class="dim small">
			${L(
				'Once a minute a connection is established with the satellites from a server in St. Petersburg, ' +
					'elapsed time is saved. Timeout is 2 s. ' +
					'Narrow red stripes are not a sign of offline: just for some reason a single response was not received.',
				'ru',
				'Раз в минуту с сателлитами устанавливается соединение из сервера в Санкт-Петербурге, ' +
					'затраченное время сохраняется. Таймаут — 2 с. ' +
					'Узкие красные полосы — не 100%-признак оффлайна: просто по какой-то причине не вернулся одиночный ответ.',
			)}
		</p>
	`
})
