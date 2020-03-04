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
import {
	CanvasExt,
	RectCenter,
	View,
	drawMonthDays,
	drawLineStepped,
	value2yLog,
	value2y,
	getArrayMaxValue,
	drawLabeledVScaleLeftLine,
	roundRange,
} from './chart_utils'
import { L, lang } from './i18n'

import './storj_tx_summary.css'

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
	let withdrawalTotal = 0
	for (let i = 0; i < payouts.length; i++) {
		if (payouts[i] > 0) {
			payoutTotal += payouts[i]
			payoutsCount += payoutCounts[i]
		}
		withdrawalTotal += withdrawals[i]
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
		aggregated: {
			payoutTotal,
			payoutAvg,
			payoutsCount,
			withdrawalTotal,
		},
	}
}

export class StorjTxSummary extends PureComponent {
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

		let now = new Date()
		this.state = {
			startDate: startOfMonth(now),
			endDate: endOfMonth(now),
			arrays: null,
			isLogScale: true,
		}
	}

	scalesLabelFunc(value) {
		if (value > 1000) return Math.round(value / 100) / 10 + L('K', 'ru', 'К')
		return Math.round(value)
	}

	loadData() {
		let { startDateStr: start, endDateStr: end } = toISODateStringInterval(this.state)
		apiReq('GET', `/api/storj_token/summary`, {
			data: { start_date: start, end_date: end },
		})
			.then(r => r.arrayBuffer())
			.then(buf => {
				let { startDate, endDate } = this.state
				let data = processTxData(buf, startDate, endDate)
				this.setState(data)
				let maxVal = getArrayMaxValue(data.arrays.payouts)
				this.view.updateLimits(...roundRange(0, maxVal))
				this.requestRedraw()
			})
			.catch(onError)
	}

	onRedraw() {
		let { canvasExt, rect, view } = this
		let { arrays, startDate, endDate, isLogScale } = this.state
		let { rc } = canvasExt

		if (!canvasExt.created()) return
		canvasExt.resize()

		rect.update(canvasExt.cssWidth, canvasExt.cssHeight)
		view.updateStamps(startDate.getTime(), endDate.getTime())

		canvasExt.clear()
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)

		drawMonthDays(canvasExt, rect, view, {})

		if (arrays !== null) {
			let start = startDate.getTime()
			let step = 3600 * 1000
			let func = isLogScale ? value2yLog : value2y
			drawLineStepped(canvasExt, rect, view, arrays.withdrawals, start, step, 'purple', true, func)
			// drawLineStepped(canvasExt, rect, view, arrays.payoutCounts, start, step, 'black', true, func)
			drawLineStepped(canvasExt, rect, view, arrays.payouts, start, step, 'green', true, func)
			drawLineStepped(canvasExt, rect, view, arrays.preparings, start, step, 'orange', true, func)
		}

		let textCol = 'black'
		let lineCol = 'rgba(0,0,0,0.08)'
		let func = isLogScale ? value2yLog : value2y
		let topVal = view.topValue
		let midVal = isLogScale ? Math.sqrt(topVal) : topVal / 2
		midVal = roundRange(0, midVal, isLogScale ? 1 : 2)[1]
		rc.textAlign = 'left'
		rc.textBaseline = 'bottom'
		drawLabeledVScaleLeftLine(rc, rect, view, 0, textCol, lineCol, 0, this.scalesLabelFunc)
		rc.textBaseline = 'middle'
		drawLabeledVScaleLeftLine(rc, rect, view, midVal, textCol, lineCol, 0, this.scalesLabelFunc, func)
		rc.textBaseline = 'top'
		drawLabeledVScaleLeftLine(rc, rect, view, topVal, textCol, lineCol, 0, this.scalesLabelFunc)

		rc.restore()
	}
	onResize() {
		this.requestRedraw()
	}

	onScaleModeClick(e) {
		this.setState({ isLogScale: !!e.target.dataset.isLog })
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

	render(props, { aggregated, isLogScale }) {
		let infoElem = '...'
		if (aggregated) {
			let total = Math.round(aggregated.payoutTotal)
			let count = Math.round(aggregated.payoutsCount)
			let avg = Math.round(aggregated.payoutAvg * 10) / 10
			let withdr = Math.round(aggregated.withdrawalTotal)
			let withdrPerc = ((withdr * 100) / total).toFixed(1)
			infoElem = html`
				<p>
					${lang == 'ru'
						? `В текущем месяце отправлено ${L.n(count, 'платёж', 'платежа', 'платежей')} ` +
						  `на ${L.n(total, 'STROJ', "STROJ'а", "STROJ'ей")}, ${avg} в среднем.`
						: `During this month ${L.n(count, 'payment', 'payments')} were sent ` +
						  `for ${L.n(total, 'STROJ', 'STROJs')}, ${avg} on average.`}
					${' '}
					${lang == 'ru'
						? `С кошельков получателей выведено ${L.n(withdr, 'койн', 'конйа', 'койнов')} ` +
						  `(${withdrPerc}% от выплат).`
						: `${L.n(withdr, 'coin', 'coins')} were withdrawn from recipient's wallets ` +
						  `(${withdrPerc}% of payouts).`}
				</p>
			`
		}
		return html`
			${infoElem}
			<div class="chart storj-tx-summary-chart">
				<canvas class="main-canvas" ref=${this.canvasExt.setRef}></canvas>
				<div class="legend">
					<${LegendItem} color="orange">${L('preparation', 'ru', 'подготовка')}</>
					<${LegendItem} color="green">${L('payouts', 'ru', 'выплаты')}</>
					<${LegendItem} color="purple">${L('withdrawals', 'ru', 'вывод')}</>
					<div class="scale-mode-wrap">
						${L('scale', 'ru', 'шкала')}:
						<button class="${isLogScale ? '' : 'active'}" onClick=${this.onScaleModeClick}>lin</button>
						<button class="${isLogScale ? 'active' : ''}" onClick=${this.onScaleModeClick} data-is-log="1">log</button>
					</div>
				</div>
			</div>
			<p class="dim small">
				<b>${L('preparation', 'ru', 'подготовка')}</b>
				${
					lang == 'ru'
						? " — входящие переводы в Storj'евый кошелёк выплат"
						: ' — incoming transfers to Storj payout wallet(s)'
				},${' '}
				<b>${L('payouts', 'ru', 'выплаты')}</b>
				${lang == 'ru' ? ' — собственно выплаты операторам нод' : ' — actual payouts to Storj Node Operators'},${' '}
				<b>${L('withdrawals', 'ru', 'вывод')}</b>
				${
					lang == 'ru'
						? ' — переводы токенов из кошельков SNO (в обменники или просто на другие адреса)'
						: ' — token transfers from SNO wallets (to exchangers or just other addresses)'
				}.
			</p>
		`
	}
}
