import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'preact/hooks'
import { apiReq } from 'src/api'
import { onError } from 'src/errors'
import { L, lang } from 'src/i18n'
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
	LegendItem,
} from 'src/utils/charts'
import { delayedRedraw, useResizeEffect } from 'src/utils/elems'
import { html } from 'src/utils/htm'
import { DAY_DURATION, toISODateString, useHashInterval } from 'src/utils/time'

const OFFICIAL_COLOR = 'hsl(30, 100%, 45%)'
function hoursColor(hours) {
	return 'hsl(240, 100%, ' + (50 + (1 - hours / 24) * 40) + '%)'
}

export function NodesCountChart() {
	const canvasExt = useRef(new CanvasExt()).current
	const [startDate, endDate] = useHashInterval()

	/**
	 * @typedef {{
	 *   startStamp: number,
	 *   counts: { h05:Int32Array, h8:Int32Array, h24:Int32Array, official:Int32Array },
	 *   changes: { left:Int32Array, come:Int32Array },
	 *   current: { count:number, lastDayInc: number, isOfficial:boolean }
	 * }} CountsData
	 */
	const [data, setData] = useState(/**@type {CountsData|null}*/ (null))

	const rect = useRef(new RectCenter({ left: 0, right: 0, top: 1, bottom: 11 })).current
	const view = useRef(new View({ startStamp: 0, endStamp: 0, bottomValue: 0, topValue: 1 })).current
	const barsView = useRef(new View({ startStamp: 0, endStamp: 0, bottomValue: 0, topValue: 1 })).current

	const onRedraw = useCallback(() => {
		let { rc } = canvasExt

		if (!canvasExt.created() || rc === null) return
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
			drawDailyComeLeftBars(rc, rect, barsView, come, left, comeCol, leftCol, '#CCC')

			const step = 3600 * 1000
			drawLineStepped(rc, rect, view, counts.official, start, step, OFFICIAL_COLOR, true, false)
			drawLineStepped(rc, rect, view, counts.h24, start, step, hoursColor(24), true, false)
			drawLineStepped(rc, rect, view, counts.h8, start, step, hoursColor(12), true, false)
			drawLineStepped(rc, rect, view, counts.h05, start, step, hoursColor(3), true, false)
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
	}, [startDate, endDate, data])
	const requestRedraw = useMemo(() => delayedRedraw(onRedraw), [onRedraw])

	useEffect(() => {
		apiReq('GET', `/api/nodes/counts`, {
			data: { start_date: toISODateString(startDate), end_date: toISODateString(endDate) },
		})
			.then(r => r.arrayBuffer())
			.then(buf => {
				const COUNTS_ITEM_SIZE = 8 / 2 //in shorts (int16)
				const CHANGES_ITEM_SIZE = 4 / 2 //in shorts (int16)
				const t = new Uint32Array(buf.slice(0, 8))
				const startStamp = t[0] * 1000
				const countsLength = t[1]
				const countsBuf = new Uint16Array(buf, 4 + 4)
				const h05 = new Int32Array(countsLength)
				const h8 = new Int32Array(countsLength)
				const h24 = new Int32Array(countsLength)
				const official = new Int32Array(countsLength)
				for (let i = 0; i < countsLength; i++) {
					h05[i] = countsBuf[i * COUNTS_ITEM_SIZE + 0]
					h8[i] = countsBuf[i * COUNTS_ITEM_SIZE + 1]
					h24[i] = countsBuf[i * COUNTS_ITEM_SIZE + 2]
					official[i] = countsBuf[i * COUNTS_ITEM_SIZE + 3]
				}

				buf = buf.slice(4 + 4 + countsLength * 8)
				const changesLength = new Uint32Array(buf.slice(0, 4))[0]
				const changesBuf = new Uint16Array(buf, 4)
				const left = new Int32Array(changesLength)
				const come = new Int32Array(changesLength)
				for (let i = 0; i < changesLength; i++) {
					come[i] = changesBuf[i * CHANGES_ITEM_SIZE + 0]
					left[i] = changesBuf[i * CHANGES_ITEM_SIZE + 1]
				}

				// Both -1 and -2 since hour counts for the last hour may be already saved
				// and official data may be not (and it'll be zero).
				// There is also no official data for old intervals.
				const curOffCount = official[official.length - 1] || official[official.length - 2]
				const current = !!curOffCount
					? {
							count: curOffCount,
							lastDayInc: curOffCount - official[Math.max(0, official.length - 24 - 1)],
							isOfficial: true,
					  }
					: {
							count: h24[h24.length - 1], //TODO
							lastDayInc: h24[h24.length - 1] - h24[Math.max(0, h24.length - 24 - 1)],
							isOfficial: false,
					  }

				setData({
					startStamp,
					counts: { h05, h8, h24, official },
					changes: { left, come },
					current,
				})
				const minVal = getArrayMinValue(h05, Infinity, true)
				const maxVal = Math.max(getArrayMaxValue(h24), getArrayMaxValue(official))
				view.updateLimits(...roundRange(minVal, maxVal))
				barsView.updateLimits(...roundRange(0, maxVal - minVal))
			})
			.catch(onError)
	}, [startDate, endDate])

	useLayoutEffect(() => {
		requestRedraw()
	}, [requestRedraw])

	useResizeEffect(requestRedraw, [requestRedraw])

	const countNow = data && data.current.count
	const countInc = data ? signed(data.current.lastDayInc) : '...'
	const countIsOfficial = data ? data.current.isOfficial : true

	const noteDate = new Date('2020-10-30T12:00:00Z')
	const note =
		startDate < noteDate && endDate > noteDate
			? lang === 'ru'
				? '750 нод добавились после исправления бага: ранее нода переставала учитываться, ' +
				  'если была в оффлайне больше недели (даже если потом снова появлялась в сети).'
				: '750 nodes were added as a result of a bugfix: previously, a node was no longer counted ' +
				  'if has been offline for more than a week (even if it later got back online).'
			: null

	return html`
		<h2>${L('Network size', 'ru', 'Размер сети')}</h2>
		<p>
			${lang === 'ru'
				? `Всего в сети
					${countIsOfficial ? '' : '~'}${L.n(countNow, 'живая нода', 'живые ноды', 'живых нод')},
					${countInc} за последний день`
				: `There are
					${countIsOfficial ? '' : '~'}${L.n(countNow, 'active node', 'active nodes')},
					${countInc} during the last day`}${' '}
			<span class="dim small">
				${countIsOfficial
					? html`(${lang === 'ru'
								? 'максимум среди спутников, по'
								: 'max among all satellites, according to'}${' '}
							<a href="https://stats.storjshare.io/" target="_blank">
								${lang === 'ru' ? 'официальным данным' : 'official data'}</a
							>)`
					: lang === 'ru'
					? `(живыми считаются ноды, к которым удалось подключиться за последние 24 часа)`
					: `(node is considered active if it was reachable within the last 24 hours)`}
			</span>
		</p>
		${note && html`<p class="dim small"><b>${noteDate.toISOString().substr(0, 10)}:</b> ${note}</p>`}
		<div class="chart">
			<canvas class="main-canvas" ref=${canvasExt.setRef}></canvas>
			<div class="legend">
				<${LegendItem} color="${OFFICIAL_COLOR}" text=${L('official', 'ru', 'офиц.')} />
				<${LegendItem} color="${hoursColor(24)}" text=${L('24 h', 'ru', '24 ч')} />
				<${LegendItem} color="${hoursColor(12)}" text=${L('8 h', 'ru', '8 ч')} />
				<${LegendItem} color="${hoursColor(3)}" text=${L('30 m', 'ru', '30 м')} />
			</div>
		</div>
		<p class="dim small">
			${lang === 'ru'
				? 'Кроме официального кол-ва тут также выводится кол-во нод, которых storjnet.info получил от спутников,' +
				  ' и к которым смог подключиться за последние N часов.'
				: 'In addition to the official count there is also the number of nodes which storjnet.info has received from satellites' +
				  ' and which were reachable within the last N hours.'}
		</p>
	`
}
