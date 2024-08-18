import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'preact/hooks'
import { apiReq } from 'src/api'
import { onError } from 'src/errors'
import {
	CanvasExt,
	drawLabeledVScaleLeftLine,
	drawLineStepped,
	drawMonthDays,
	getArrayMaxValue,
	LegendItem,
	RectCenter,
	roundRange,
	View,
} from 'src/utils/charts'
import { delayedRedraw, useResizeEffect } from 'src/utils/elems'
import { html } from 'src/utils/htm'
import { DAY_DURATION, toISODateString, useHashInterval } from 'src/utils/time'

import './node_countries_chart.css'
import { lang } from 'src/i18n'

/** @typedef {{name:string, a3:string, counts:Uint16Array}} CountryItem */

function c(str, n) {
	return str.charCodeAt(n) || 0
}

/** @param {string} name */
function name2col(name) {
	// const h = (3600 + 90 - 7 * c(name, 0) - 3 * c(name, 1) + 11 * c(name, 2)) % 360
	// let s = 90 - 50 * ((c(name, 0) / 7) % 1) ** 2
	// let l = 55 - 25 * ((c(name, 0) / 11) % 1) ** 2
	// l = Math.min(l, 50 - 15 * Math.max(0, Math.sin(((h - 45) / 180) * Math.PI)) ** 2)
	// return `hsl(${h} ${s}% ${l}%)`
	const sum = c(name, 0) + c(name, 1) + c(name, 2)
	const max = Math.max(c(name, 0), c(name, 1), c(name, 2))
	let r = (max * 1.0031 * c(name, 0) + sum * c(name, 2) + max * c(name, 1)) % 256 | 0
	let g = (max * 1.0072 * c(name, 1) + sum * c(name, 0) + max * c(name, 2)) % 256 | 0
	let b = (max * 1.0025 * c(name, 2) + sum * c(name, 1) + max * c(name, 0)) % 256 | 0

	const lum = r * 0.21 + g * 0.72 + b * 0.07
	let dLum = 0
	if (lum > 150) dLum = -130
	if (lum < 120) dLum = 120 - lum * 0.75
	if (dLum !== 0) {
		r = Math.max(0, r + dLum * 0.21) | 0
		g = Math.max(0, g + dLum * 0.72) | 0
		b = Math.max(0, b + dLum * 0.07) | 0
	}
	return `rgb(${r},${g},${b})`
}

export function NodeCountriesChart() {
	const canvasExt = useRef(new CanvasExt()).current
	const legendBoxRef = useRef(/**@type {HTMLDivElement|null}*/ (null))
	const [startDate, endDate] = useHashInterval()

	const [data, setData] = useState(/**@type {{startStamp:number, countries:CountryItem[]}|null}*/ (null))

	const rect = useRef(new RectCenter({ left: 0, right: 0, top: 31, bottom: 11 })).current
	const view = useRef(new View({ startStamp: 0, endStamp: 0, bottomValue: 0, topValue: 1 })).current

	const onRedraw = useCallback(() => {
		let { rc } = canvasExt

		if (!canvasExt.created() || rc === null) return
		canvasExt.resize()

		if (legendBoxRef.current) rect.top = legendBoxRef.current.getBoundingClientRect().height + 1
		rect.update(canvasExt.cssWidth, canvasExt.cssHeight)
		view.updateStamps(+startDate, +endDate + DAY_DURATION)

		canvasExt.clear()
		rc.save()
		rc.scale(canvasExt.pixelRatio, canvasExt.pixelRatio)
		rc.lineWidth = 1.2

		if (data !== null) {
			const { startStamp: start, countries } = data
			const step = 3600 * 1000
			for (const country of countries) {
				const col = name2col(country.a3)
				drawLineStepped(rc, rect, view, country.counts, start, step, col, true, false) //value2yLog
			}
		}

		const textCol = 'black'
		const lineCol = 'rgba(0,0,0,0.08)'
		const midVal = (view.bottomValue + view.topValue) / 2
		rc.textAlign = 'left'
		rc.textBaseline = 'bottom'
		drawLabeledVScaleLeftLine(rc, rect, view, view.bottomValue, textCol, null, 0, null)
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
		apiReq('GET', `/api/nodes/countries`, {
			data: { start_date: toISODateString(startDate), end_date: toISODateString(endDate), lang },
			signal: abortController.signal,
		})
			.then(r => r.arrayBuffer())
			.then(arrayBuf => {
				const uint32Buf = new Uint32Array(arrayBuf, 0, 2)
				const startStamp = uint32Buf[0] * 1000
				const countsLength = uint32Buf[1]

				/** @type {CountryItem[]} */
				const countries = []

				const textDec = new TextDecoder()
				const buf = new Uint8Array(arrayBuf)
				let pos = 8
				let maxCount = 0
				while (pos < buf.length) {
					const a3AndNameLen = buf[pos]
					const a3AndName = textDec.decode(new Uint8Array(arrayBuf, pos + 1, a3AndNameLen))
					const [a3, name] = a3AndName.split('|')
					let countsOffset = 1 + a3AndNameLen
					if (countsOffset % 2 === 1) countsOffset += 1
					pos += countsOffset
					const counts = new Uint16Array(arrayBuf, pos, countsLength)
					pos += 2 * countsLength
					countries.push({ name, a3, counts })
					maxCount = Math.max(maxCount, getArrayMaxValue(counts))
				}

				countries.sort((a, b) => b.counts[countsLength - 1] - a.counts[countsLength - 1])

				setData({ startStamp, countries })
				view.updateLimits(...roundRange(0, maxCount))
			})
			.catch(onError)
		return () => abortController.abort()
	}, [startDate, endDate])

	useLayoutEffect(() => {
		requestRedraw()
	}, [requestRedraw])

	useResizeEffect(requestRedraw, [requestRedraw])

	return html`
		<div class="chart node-countries-chart">
			<canvas class="main-canvas" ref=${canvasExt.setRef}></canvas>
			<div class="legend leftmost" ref=${legendBoxRef}>
				${data !== null &&
				data.countries.map(x => html`<${LegendItem} color="${name2col(x.a3)}">${x.name}<//>`)}
			</div>
		</div>
	`
}
