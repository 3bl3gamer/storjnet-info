import { apiReq } from 'src/api'
import { L, lang } from 'src/i18n'
import { memo, PureComponent } from 'src/utils/preact_compat'
import { onError } from 'src/errors'
import { html } from 'src/utils/htm'
import { zeroes } from 'src/utils/arrays'
import { intervalIsDefault, toISODateString, useHashInterval, watchHashInterval } from 'src/utils/time'
import {
	LocMap,
	ProjectionMercator,
	oneOf,
	TileLayer,
	ControlLayer,
	appendCredit,
	ControlHintLayer,
	SmoothTileContainer,
	clampEarthTiles,
	loadTileImage,
} from 'locmap'
import { PointsLayer } from 'src/map_points_layer'
import { NodeCountriesChart } from './node_countries_chart'
import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks'

import './nodes_location_summary.css'

const NodesLocationMap = memo(function NodesLocationMap() {
	const mapPointsLayer = useRef(new PointsLayer()).current
	const mapWrapRef = useRef(/**@type {HTMLDivElement|null}*/ (null))

	useEffect(() => {
		apiReq('GET', `/api/nodes/locations`)
			.then(r => r.arrayBuffer())
			.then(buf => {
				const coords = new Uint16Array(buf)
				const locations = new Array(coords.length / 2)
				for (let i = 0; i < coords.length / 2; i++)
					locations[i] = [
						(coords[i * 2 + 0] / 65536) * 360 - 180,
						(coords[i * 2 + 1] / 65536) * 180 - 90,
					]
				mapPointsLayer.setLocations(locations)
			})
			.catch(onError)
	}, [])

	useEffect(() => {
		if (!mapWrapRef.current) return

		const map = new LocMap(mapWrapRef.current, ProjectionMercator)
		map.updateLocation(0, 34, map.getCanvas().getBoundingClientRect().width)

		const tileContainer = new SmoothTileContainer(
			256,
			clampEarthTiles(
				loadTileImage((x, y, z) => {
					//return `https://${oneOf('a','b','c','d')}.basemaps.cartocdn.com/rastertiles/dark_all/${z}/${x}/${y}@1x.png`
					const s = oneOf('a', 'b', 'c', 'd')
					return `https://${s}.basemaps.cartocdn.com/rastertiles/light_all/${z}/${x}/${y}@1x.png`
				}),
			),
		)
		map.register(new TileLayer(tileContainer))
		map.register(new ControlLayer({ doNotInterfere: true }))
		appendCredit(
			mapWrapRef.current,
			'© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors' +
				' © <a href="https://carto.com/attributions">CARTO</a>',
		)
		map.register(mapPointsLayer)
		const controlText = L('hold Ctrl to zoom', 'ru', 'зажмите Ctrl для зума')
		const twoFingersText = L('use two fingers to drag', 'ru', 'для перетаскивания жмите двумя пальцами')
		map.register(new ControlHintLayer(controlText, twoFingersText))
		map.resize()

		addEventListener('resize', map.resize)

		return () => {
			map.unregister(mapPointsLayer)
			removeEventListener('resize', map.resize)
		}
	}, [])

	return html`<div class="map-wrap" ref=${mapWrapRef}></div>`
})

const NodesSummary = memo(function NodesSummary() {
	const [stats, setStats] = useState(
		/**
		 * @type {{
		 *   countriesCount: number,
		 *   countriesTop: {country:string, nodes:number, subnets:number}[]
		 * } | null}
		 */ (null),
	)
	const [isExpanded, setIsExpanded] = useState(false)
	const [sortCol, setSortCol] = useState(/**@type {'nodes'|'subnets'|'nodesPerSub'}*/ ('nodes'))
	const [, endDate] = useHashInterval()

	useEffect(() => {
		const abortController = new AbortController()

		apiReq('GET', `/api/nodes/location_summary`, {
			data: { end_date: toISODateString(endDate) },
			signal: abortController.signal,
		})
			.then(stats => {
				setStats(stats)
			})
			.catch(onError)

		return () => abortController.abort()
	}, [endDate])

	const onExpand = useCallback(() => {
		setIsExpanded(true)
	}, [])

	const countriesTopSorted = useMemo(() => {
		if (stats === null) return null
		return stats.countriesTop.sort(
			sortCol === 'nodesPerSub'
				? (a, b) => b.nodes / b.subnets - a.nodes / a.subnets
				: (a, b) => b[sortCol] - a[sortCol],
		)
	}, [stats, sortCol])

	const countriesCount = stats && stats.countriesCount
	return html`
		<div class="p-like">
			<table class="node-countries-table underlined wide-padded">
				<thead>
					<tr>
						<td>${L('#', 'ru', '№')}</td>
						<td>${L('Country', 'ru', 'Страна')}</td>
						<td>
							<button class="a-like" onclick=${() => setSortCol('nodes')}>
								${L('Nodes', 'ru', 'Ноды')}${sortCol === 'nodes' ? '▼' : ''}
							</button>
						</td>
						<td>
							<button class="a-like" onclick=${() => setSortCol('subnets')}>
								${L('Subnets', 'ru', 'Подсети')}${sortCol === 'subnets' ? '▼' : ''}
							</button>
						</td>
						<td>
							<button class="a-like" onclick=${() => setSortCol('nodesPerSub')}>
								${L('Avg', 'ru', 'Сред.')}${sortCol === 'nodesPerSub' ? '▼' : ''}
							</button>
						</td>
					</tr>
				</thead>
				<tbody>
					${countriesTopSorted === null
						? zeroes(10).map(
								(_, i) => html`<tr>
									<td class="dim">${i + 1}</td>
									<td class="dim">${L('loading...', 'ru', 'загрузка...')}</td>
									<td class="dim">...</td>
									<td class="dim">...</td>
									<td class="dim">...</td>
								</tr>`,
						  )
						: (isExpanded ? countriesTopSorted : countriesTopSorted.slice(0, 10)).map(
								(item, i) =>
									html`<tr>
										<td>${i + 1}</td>
										<td>${item.country}</td>
										<td class=${sortCol === 'nodes' ? '' : 'dim'}>
											${blankIfZero(item.nodes)}
										</td>
										<td class=${sortCol === 'subnets' ? '' : 'dim'}>
											${blankIfZero(item.subnets)}
										</td>
										<td class=${sortCol === 'nodesPerSub' ? '' : 'dim'}>
											${item.subnets === 0
												? ''
												: (item.nodes / item.subnets).toFixed(1)}
										</td>
									</tr>`,
						  )}
					<tr>
						${!isExpanded &&
						html`
							<td colspan="4">
								<button class="a-like" onclick=${onExpand}>
									${L('Expand', 'ru', 'Развернуть')}
								</button>
							</td>
						`}
					</tr>
				</tbody>
			</table>
		</div>
		<p>
			${lang === 'ru'
				? `Ноды запущены как минимум в ${L.n(countriesCount, 'стране', 'странах', 'странах')}.`
				: `Nodes are running in at least ${L.n(countriesCount, 'country', 'countries')}.`}
		</p>
	`
})

export class NodesLocationSummary extends PureComponent {
	constructor() {
		super()

		let watch = watchHashInterval((startDate, endDate) => {
			this.setState({ ...this.state, intervalIsDefault: intervalIsDefault() })
		})
		this.stopWatchingHashInterval = watch.off

		this.state = { intervalIsDefault: intervalIsDefault() }
	}

	componentWillUnmount() {
		this.stopWatchingHashInterval()
	}

	render(props, { intervalIsDefault }) {
		return html`
			<h3>${L('By countries', 'ru', 'По странам')}</h3>
			<${NodeCountriesChart} />
			<${NodesSummary} />
			<h2>${L('Nodes location', 'ru', 'Расположение нод')}</h2>
			${!intervalIsDefault &&
			html`<p class="warn">
				${lang === 'ru'
					? 'Для местоположений перемотка не работает. Пока.'
					: 'Locations can not rewind. Yet.'}
			</p>`}
			<${NodesLocationMap} />
		`
	}
}

/** @param {number} num */
function blankIfZero(num) {
	return num === 0 ? '' : num + ''
}
