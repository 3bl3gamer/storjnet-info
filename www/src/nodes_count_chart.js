import {
	PureComponent,
	delayedRedraw,
	bindHandlers,
	toISODateStringInterval,
	onError,
	html,
	LegendItem,
	watchHashInterval,
	DAY_DURATION,
} from './utils'
import {
	View,
	RectCenter,
	CanvasExt,
	drawMonthDays,
	drawLabeledVScaleLeftLine,
	getArrayMaxValue,
	roundRange,
	drawLineStepped,
	getArrayMinValue,
	drawDailyComeLeftBars,
	signed,
} from './chart_utils'
import { apiReq } from './api'
import { L, lang } from './i18n'

function hoursColor(hours) {
	return 'hsl(240, 100%, ' + (50 + (1 - hours / 24) * 40) + '%)'
}

export class NodesCountChart extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.canvasExt = new CanvasExt()

		this.requestRedraw = delayedRedraw(this.onRedraw)

		this.rect = new RectCenter({ left: 0, right: 0, top: 1, bottom: 11 })
		this.view = new View({
			startStamp: 0,
			endStamp: 0,
			bottomValue: 0,
			topValue: 1,
		})
		this.barsView = new View({
			startStamp: 0,
			endStamp: 0,
			bottomValue: 0,
			topValue: 1,
		})

		let watch = watchHashInterval((startDate, endDate) => {
			this.setState({ ...this.state, startDate, endDate, data: null }, () => this.loadData())
		})
		this.stopWatchingHashInterval = watch.off

		this.state = {
			startDate: watch.startDate,
			endDate: watch.endDate,
			data: null,
		}
	}

	loadData() {
		let { startDateStr: start, endDateStr: end } = toISODateStringInterval(this.state)
		apiReq('GET', `/api/nodes/counts`, {
			data: { start_date: start, end_date: end },
		})
			.then(r => r.arrayBuffer())
			.then(buf => {
				const t = new Uint32Array(buf.slice(0, 8))
				const startStamp = t[0] * 1000
				const countsLength = t[1]
				const countsBuf = new Uint16Array(buf, 4 + 4)
				const h05 = new Int32Array(countsLength)
				const h8 = new Int32Array(countsLength)
				const h24 = new Int32Array(countsLength)
				for (let i = 0; i < countsLength; i++) {
					h05[i] = countsBuf[i * 3 + 0]
					h8[i] = countsBuf[i * 3 + 1]
					h24[i] = countsBuf[i * 3 + 2]
				}

				buf = buf.slice(4 + 4 + countsLength * 6)
				const changesLength = new Uint32Array(buf.slice(0, 4))[0]
				const changesBuf = new Uint16Array(buf, 4)
				const left = new Int32Array(changesLength)
				const come = new Int32Array(changesLength)
				for (let i = 0; i < changesLength; i++) {
					come[i] = changesBuf[i * 2 + 0]
					left[i] = changesBuf[i * 2 + 1]
				}

				this.setState({
					data: {
						startStamp,
						counts: { h05, h8, h24 },
						changes: { left, come },
						currentCount: h24[h24.length - 1],
						lastDayInc: h24[h24.length - 1] - h24[Math.max(0, h24.length - 24 - 1)],
					},
				})
				const minVal = getArrayMinValue(h05, Infinity, true)
				const maxVal = getArrayMaxValue(h24)
				this.view.updateLimits(...roundRange(minVal, maxVal))
				this.barsView.updateLimits(...roundRange(0, maxVal - minVal))
				this.requestRedraw()
			})
			.catch(onError)
	}

	onRedraw() {
		let { canvasExt, rect, view, barsView } = this
		let { startDate, endDate, data } = this.state
		let { rc } = canvasExt

		if (!canvasExt.created()) return
		canvasExt.resize()

		rect.update(canvasExt.cssWidth, canvasExt.cssHeight)
		view.updateStamps(startDate.getTime(), endDate.getTime() + DAY_DURATION)
		barsView.updateStamps(startDate.getTime(), endDate.getTime() + DAY_DURATION)

		canvasExt.clear()
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)
		rc.lineWidth = 1.2

		if (data !== null) {
			const { startStamp: start, counts, changes } = data
			const { come, left } = changes

			const comeCol = 'rgba(0,200,0,0.25)'
			const leftCol = 'rgba(255,0,0,0.18)'
			drawDailyComeLeftBars(canvasExt, rect, barsView, come, left, comeCol, leftCol, '#CCC')

			const step = 3600 * 1000
			drawLineStepped(canvasExt, rect, view, counts.h24, start, step, hoursColor(24), true, false)
			drawLineStepped(canvasExt, rect, view, counts.h8, start, step, hoursColor(12), true, false)
			drawLineStepped(canvasExt, rect, view, counts.h05, start, step, hoursColor(3), true, false)
		}

		const textCol = 'black'
		const lineCol = 'rgba(0,0,0,0.08)'
		const midVal = (view.bottomValue + view.topValue) / 2
		rc.textAlign = 'left'
		rc.textBaseline = 'bottom'
		drawLabeledVScaleLeftLine(rc, rect, view, view.bottomValue, textCol, lineCol, 0, null)
		rc.textBaseline = 'middle'
		drawLabeledVScaleLeftLine(rc, rect, view, midVal, textCol, lineCol, 0, null)
		rc.textBaseline = 'top'
		drawLabeledVScaleLeftLine(rc, rect, view, view.topValue, textCol, lineCol, 0, null)

		drawMonthDays(canvasExt, rect, view, {})

		rc.restore()
	}
	onResize() {
		this.requestRedraw()
	}

	componentDidMount() {
		this.requestRedraw()
		this.loadData()
		addEventListener('resize', this.onResize)
	}
	componentWillUnmount() {
		removeEventListener('resize', this.onResize)
		this.stopWatchingHashInterval()
	}

	render(params, { data }) {
		const countNow = data && data.currentCount
		const countInc = data ? signed(data.lastDayInc) : '...'
		return html`
			<p>${
				lang === 'ru'
					? `Всего в сети ~${L.n(countNow, 'живая нода', 'живые ноды', 'живых нод')},
						${countInc} за последний день`
					: `There are ~${L.n(countNow, 'active node', 'active nodes')},
						${countInc} during the last day`
			}${' '}
				<span class="dim small">${
					lang === 'ru'
						? `(живыми считаются ноды, к которым удалось подключиться за последние 24 часа)`
						: `(node is considered active if it was reachable within the last 24 hours)`
				}</span>
			</p>
			<div class="chart storj-tx-summary-chart">
				<canvas class="main-canvas" ref=${this.canvasExt.setRef}></canvas>
				<div class="legend">
					<${LegendItem} color="${hoursColor(24)}">${L('24 h', 'ru', '24 ч')}</${LegendItem}>
					<${LegendItem} color="${hoursColor(12)}">${L('8 h', 'ru', '8 ч')}</${LegendItem}>
					<${LegendItem} color="${hoursColor(3)}">${L('30 m', 'ru', '30 м')}</${LegendItem}>
				</div>
			</div>
		`
	}
}
