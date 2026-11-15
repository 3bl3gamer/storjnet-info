import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'preact/hooks'
import { apiReq } from 'src/api'
import { onError } from 'src/errors'
import { L, lang, pluralize } from 'src/i18n'
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
import { SAT_GROUP_ALL, SAT_GROUP_OFFICIAL, getSatGroups, useHashSatGroup } from 'src/utils/sat_groups'

const REPORTED_COLOR = 'hsl(30, 100%, 45%)'
const OFFERED_COLOR = 'hsl(290, 60%, 50%)'

/** Where satellite operators publish their stats (the source of the "reported" count), by satellites group */
const OPERATOR_STATS_URLS = {
	[SAT_GROUP_OFFICIAL]: 'https://stats.storjshare.io/',
	storkspace: 'https://stats.storkspace.net/',
}

/**
 * @param {{satGroup:string}} props
 */
function SatStatURLs({ satGroup }) {
	const groups = satGroup === SAT_GROUP_ALL ? getSatGroups() : [satGroup]
	const urls = groups.map(x => OPERATOR_STATS_URLS[x]).filter(x => !!x)
	return urls.map(
		(url, i) => html`${i > 0 && ', '}<a href="${url}" target="_blank">${new URL(url).hostname}</a>`,
	)
}

function hoursColor(hours) {
	return 'hsl(240, 100%, ' + (50 + (1 - hours / 24) * 40) + '%)'
}

export function NodesCountChart() {
	const canvasExt = useRef(new CanvasExt()).current
	const [startDate, endDate] = useHashInterval()
	const satGroup = useHashSatGroup()
	const isGroup = satGroup !== SAT_GROUP_ALL

	/**
	 * `reported` is the count reported by the satellites themselves,
	 * `offered30d` is the number of nodes the satellites have offered within the last 30 days.
	 * @typedef {{
	 *   satGroup: string,
	 *   startStamp: number,
	 *   counts: { h05:Int32Array, h8:Int32Array, h24:Int32Array, reported:Int32Array, offered30d:Int32Array },
	 *   changes: { left:Int32Array, come:Int32Array },
	 *   current: { count:number, lastDayInc: number, isReported:boolean }
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
			// "all" does not depend on what specific satellites offer, so the line is only for the groups
			if (data.satGroup !== SAT_GROUP_ALL)
				drawLineStepped(rc, rect, view, counts.offered30d, start, step, OFFERED_COLOR, true, false)
			drawLineStepped(rc, rect, view, counts.reported, start, step, REPORTED_COLOR, true, false)
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
		const abortController = new AbortController()
		apiReq('GET', `/api/nodes/counts`, {
			data: {
				start_date: toISODateString(startDate),
				end_date: toISODateString(endDate),
				sat_group: satGroup,
				// the server refuses requests without it, see HandleAPINodesCounts
				format: 'v2',
			},
			signal: abortController.signal,
		})
			.then(r => r.arrayBuffer())
			.then(buf => {
				const COUNTS_ITEM_SIZE = 10 / 2 //in shorts (int16)
				const CHANGES_ITEM_SIZE = 4 / 2 //in shorts (int16)
				// ensuring lengths are correct (in case of stale page after server update)
				const assertFits = (/**@type {number}*/ bytes) => {
					if (bytes > buf.byteLength) throw new Error('nodes counts: unexpected response format')
				}
				assertFits(4 + 4)
				const t = new Uint32Array(buf.slice(0, 8))
				const startStamp = t[0] * 1000
				const countsLength = t[1]
				assertFits(4 + 4 + countsLength * COUNTS_ITEM_SIZE * 2 + 4)
				const countsBuf = new Uint16Array(buf, 4 + 4, countsLength * COUNTS_ITEM_SIZE)
				const h05 = new Int32Array(countsLength)
				const h8 = new Int32Array(countsLength)
				const h24 = new Int32Array(countsLength)
				const reported = new Int32Array(countsLength)
				const offered30d = new Int32Array(countsLength)
				for (let i = 0; i < countsLength; i++) {
					h05[i] = countsBuf[i * COUNTS_ITEM_SIZE + 0]
					h8[i] = countsBuf[i * COUNTS_ITEM_SIZE + 1]
					h24[i] = countsBuf[i * COUNTS_ITEM_SIZE + 2]
					reported[i] = countsBuf[i * COUNTS_ITEM_SIZE + 3]
					offered30d[i] = countsBuf[i * COUNTS_ITEM_SIZE + 4]
				}

				buf = buf.slice(4 + 4 + countsLength * COUNTS_ITEM_SIZE * 2)
				const changesLength = new Uint32Array(buf.slice(0, 4))[0]
				assertFits(4 + changesLength * CHANGES_ITEM_SIZE * 2)
				const changesBuf = new Uint16Array(buf, 4, changesLength * CHANGES_ITEM_SIZE)
				const left = new Int32Array(changesLength)
				const come = new Int32Array(changesLength)
				for (let i = 0; i < changesLength; i++) {
					come[i] = changesBuf[i * CHANGES_ITEM_SIZE + 0]
					left[i] = changesBuf[i * CHANGES_ITEM_SIZE + 1]
				}

				// Both -1 and -2 since hour counts for the last hour may be already saved
				// and satellites data may be not (and it'll be zero).
				// There is also no satellites data for old intervals.
				const curRepCount = reported[reported.length - 1] || reported[reported.length - 2]
				const current = !!curRepCount
					? {
							count: curRepCount,
							lastDayInc: curRepCount - reported[Math.max(0, reported.length - 24 - 1)],
							isReported: true,
						}
					: {
							count: h24[h24.length - 1], //TODO
							lastDayInc: h24[h24.length - 1] - h24[Math.max(0, h24.length - 24 - 1)],
							isReported: false,
						}

				setData({
					satGroup,
					startStamp,
					counts: { h05, h8, h24, reported, offered30d },
					changes: { left, come },
					current,
				})
				// h05 <= h8 <= h24, no need to check them all
				const lowLines = isGroup ? [h05, reported, offered30d] : [h05, reported]
				const highLines = isGroup ? [h24, reported, offered30d] : [h24, reported]
				let minVal = Math.min(...lowLines.map(x => getArrayMinValue(x, Infinity, true)))
				let maxVal = Math.max(...highLines.map(x => getArrayMaxValue(x)))
				// no data at all (the group may have no stats for the interval)
				if (minVal === Infinity) minVal = 0
				// range must not be empty
				if (maxVal <= minVal) maxVal = minVal + 2
				view.updateLimits(...roundRange(minVal, maxVal))
				barsView.updateLimits(...roundRange(0, maxVal - minVal))
			})
			.catch(onError)
		return () => abortController.abort()
	}, [startDate, endDate, satGroup])

	useLayoutEffect(() => {
		requestRedraw()
	}, [requestRedraw])

	useResizeEffect(requestRedraw, [requestRedraw])

	// while the data of the just chosen group is being loaded, the old numbers must not go with the new group name
	const textData = data !== null && data.satGroup === satGroup ? data : null
	const countNow = textData && textData.current.count
	const countInc = textData ? signed(textData.current.lastDayInc) : '...'
	const countIsReported = textData ? textData.current.isReported : true

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
				? `Как минимум ${countIsReported ? '' : '~'}${L.n(countNow, 'живая нода', 'живых ноды', 'живых нод')}, ${countInc} за последний день`
				: `At least ${countIsReported ? '' : '~'}${L.n(countNow, 'active node', 'active nodes')}, ${countInc} during the last day`}${' '}
			<span class="dim small">
				${countIsReported
					? html`(${lang === 'ru' ? 'максимум по данным ' : 'max among '}
							<${SatStatURLs} satGroup=${satGroup} />)`
					: lang === 'ru'
						? `(живыми считаются ноды, к которым удалось подключиться за последние 24 часа)`
						: `(node is considered active if it was reachable within the last 24 hours)`}
			</span>
		</p>
		${note && html`<p class="dim small"><b>${noteDate.toISOString().slice(0, 10)}:</b> ${note}</p>`}
		<div class="chart">
			<canvas class="main-canvas" ref=${canvasExt.setRef}></canvas>
			<div class="legend">
				<${LegendItem} color="${REPORTED_COLOR}" text=${L('public', 'ru', 'публичное')} />
				<${LegendItem} color="${hoursColor(24)}" text=${L('24 h', 'ru', '24 ч')} />
				<${LegendItem} color="${hoursColor(12)}" text=${L('8 h', 'ru', '8 ч')} />
				<${LegendItem} color="${hoursColor(3)}" text=${L('30 m', 'ru', '30 м')} />
				${isGroup &&
				html`<${LegendItem}
					color="${OFFERED_COLOR}"
					text=${L('offered in 30 d', 'ru', 'выдавались за 30 д')}
				/>`}
			</div>
		</div>
		<p class="dim small">
			${lang === 'ru'
				? html`<b>публичное</b> кол-во получено со страниц открытой статистики спутников
						(<${SatStatURLs} satGroup=${satGroup} />), <b>N ч</b> — кол-во нод, которые
						storjnet.info когда-либо получил от спутников и к которым смог подключиться за
						последние N
						часов${isGroup
							? html`, <b>выдавались за N</b> — полученные от спутников ноды независимо от их
									живости`
							: ''}.`
				: html`<b>public</b> was obtained from the satellites open statistics pages (<${SatStatURLs}
							satGroup=${satGroup}
						/>), <b>N h</b> is the number of nodes which storjnet.info has received from
						satellites and which were reachable within the last N
						hours${isGroup
							? html`, <b>offered in N</b> is the number of nodes received from satellites,
									regardless of their activeness`
							: ''}.`}
		</p>
	`
}
