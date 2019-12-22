import {
	PureComponent,
	html,
	startOfMonth,
	endOfMonth,
	toISODateStringInterval,
	onError,
	bindHandlers,
	delayedRedraw,
	LegendItem,
} from './utils'
import { apiReq } from './api'
import { CanvasExt, RectCenter, View, drawMonthDays, drawLineStepped } from './chart_utils'
import { L, lang } from './i18n'

function processTxData(buf, startDate, endDate) {
	let startStamp = Math.floor(startDate.getTime())
	let endStamp = Math.floor(endDate.getTime())

	let dayByteSize = 4 + 24 * (4 + 4 + 4 + 4)
	let daysCount = buf.byteLength / dayByteSize

	let len = Math.floor((endStamp - startStamp) / 3600 / 1000)
	let preparings = new Float32Array(len)
	let payouts = new Float32Array(len)
	let payoutCounts = new Int32Array(len)
	let withdrawals = new Float32Array(len)

	for (let i = 0; i < daysCount; i++) {
		let bufOffset = i * dayByteSize
		let ints = new Int32Array(buf, bufOffset)
		let floats = new Float32Array(buf, bufOffset)
		let stamp = ints[0] * 1000
		let offset = Math.floor((stamp - startStamp) / 3600 / 1000)
		let iFrom = offset < 0 ? -offset : 0
		let iTo = offset + 24 > len ? len - offset : 24
		for (let i = iFrom; i < iTo; i++) {
			preparings[offset + i] += floats[1 + 24 * 0 + i]
			payouts[offset + i] += floats[1 + 24 * 1 + i]
			payoutCounts[offset + i] += ints[1 + 24 * 2 + i]
			withdrawals[offset + i] += floats[1 + 24 * 3 + i]
		}
	}

	let payoutTotal = 0
	let payoutsCount = 0
	for (let i = 0; i < payouts.length; i++) {
		if (payouts[i] > 0) {
			payoutTotal += payouts[i]
			payoutsCount += payoutCounts[i]
		}
	}
	let payoutAvg = payoutsCount == 0 ? 0 : payoutTotal / payoutsCount

	for (let i = withdrawals.length - 1; i >= 1; i--) {
		withdrawals[i] = (withdrawals[i] + withdrawals[i - 1]) / 2
	}

	return {
		arrays: {
			preparings,
			payouts,
			payoutCounts,
			withdrawals,
		},
		aggregated: { payoutTotal, payoutAvg, payoutsCount },
	}
}

export class StorjTxSummary extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.canvasExt = new CanvasExt()

		this.requestRedraw = delayedRedraw(this.onRedraw)

		this.rect = new RectCenter({ left: 0, right: 0, top: 0, bottom: 11 })
		this.view = new View({
			startStamp: 0,
			endStamp: 0,
			bottomValue: 0,
			topValue: 1000000,
		})

		let now = new Date()
		this.state = {
			startDate: startOfMonth(now),
			endDate: endOfMonth(now),
			arrays: null,
		}
	}

	loadData() {
		let { startDateStr: start, endDateStr: end } = toISODateStringInterval(this.state)
		apiReq('GET', `/api/storj_token/summary`, {
			data: { start_date: start, end_date: end },
		})
			.then(r => r.arrayBuffer())
			.then(buf => {
				let { startDate, endDate } = this.state
				this.setState(processTxData(buf, startDate, endDate))
				this.requestRedraw()
			})
			.catch(onError)
	}

	onRedraw() {
		let { canvasExt, rect, view } = this
		let { arrays, startDate, endDate } = this.state
		let { rc } = canvasExt

		if (!canvasExt.created()) return
		canvasExt.resize()

		rect.update(canvasExt.cssWidth, canvasExt.cssHeight)
		view.updateStamps(startDate.getTime(), endDate.getTime())

		canvasExt.clear()
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)

		if (arrays !== null) {
			drawLineStepped(canvasExt, rect, view, arrays.withdrawals, +startDate, 3600 * 1000, 'Coral', true)
			// drawLineStepped(canvasExt, rect, view, arrays.payoutCounts, +startDate, 3600*1000, 'black', true)
			drawLineStepped(canvasExt, rect, view, arrays.payouts, +startDate, 3600 * 1000, 'green', true)
			drawLineStepped(
				canvasExt,
				rect,
				view,
				arrays.preparings,
				+startDate,
				3600 * 1000,
				'DarkGray',
				true,
			)
		}

		drawMonthDays(canvasExt, rect, view, {})

		// rc.strokeStyle = 'rgba(0,0,0,0.05)'
		// rc.lineWidth = 0.5
		// rc.strokeRect(0.5, 0.5, canvasExt.cssWidth - 1, canvasExt.cssHeight - 1)

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
		addEventListener('resize', this.onResize)
	}

	render(props, { aggregated }) {
		let infoElem = '...'
		if (aggregated) {
			let total = Math.round(aggregated.payoutTotal)
			let count = Math.round(aggregated.payoutsCount)
			let avg = Math.round(aggregated.payoutAvg * 10) / 10
			//prettier-ignoree
			infoElem = html`
				<p>
					${lang == 'ru'
						? `За последний месяц отправлено ${L.n(count, 'платёж', 'платежа', 'платежей')} ` +
						  `на ${L.n(total, 'STROJ', "STROJ'а", "STROJ'ей")}, ${avg} в среднем.`
						: `Over the past month ${L.n(count, 'payment', 'payments')} were sent ` +
						  `for ${L.n(total, 'STROJ', "STROJs")}, ${avg} on average.`}
				</p>
			`
		}
		return html`
			${infoElem}
			<div class="chart storj-tx-summary-chart">
				<canvas class="main-canvas" ref=${this.canvasExt.setRef}></canvas>
				<div class="legend">
					<${LegendItem} color="darkgray">${L('preparation', 'ru', 'подготовка')}</>
					<${LegendItem} color="green">${L('payouts', 'ru', 'выплаты')}</>
					<${LegendItem} color="coral">${L('withdrawal', 'ru', 'вывод')}</>
				</div>
			</div>
		`
	}
}
