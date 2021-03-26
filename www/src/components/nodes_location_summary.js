import { createRef } from 'preact'

import { apiReq } from 'src/api'
import { L, lang } from 'src/i18n'
import { bindHandlers } from 'src/utils/elems'
import { PureComponent } from 'src/utils/preact_compat'
import { onError } from 'src/errors'
import { html } from 'src/utils/htm'
import { zeroes } from 'src/utils/arrays'
import { intervalIsDefault, watchHashInterval } from 'src/utils/time'

import { TileMap } from 'src/map/core/map'
import { MapTileContainer } from 'src/map/core/tile_container'
import { MapTileLayer } from 'src/map/core/tile_layer'
import { MapControlLayer, MapControlHintLayer } from 'src/map/core/control_layer'
import { PointsLayer } from 'src/map/points_layer'

import './nodes_location_summary.css'

class NodesLocationMap extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)

		this.state = { locations: [] }

		this.mapWrapRef = createRef()
		this.map = null
	}

	loadData() {
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
				this.setState({ locations })
				this.mapPointsLayer.setLocations(locations)
			})
			.catch(onError)
	}

	setupMap() {
		const map = new TileMap(this.mapWrapRef.current, TileMap.proj.Mercator)
		map.updateLocation(0, 34, Math.log2(map.canvas.getBoundingClientRect().width))
		if (map.top_left_y_shift < 0) map.move(0, map.top_left_y_shift)

		const tileContainer = new MapTileContainer(
			256,
			(x, y, z) => {
				//return `https://c.basemaps.cartocdn.com/rastertiles/dark_all/${z}/${x}/${y}@1x.png`
				return `https://c.basemaps.cartocdn.com/rastertiles/light_all/${z}/${x}/${y}@1x.png`
			},
			TileMap.proj.Mercator,
		)
		map.register(new MapTileLayer(tileContainer))
		map.register(new MapControlLayer(map))
		const pointsLayer = new PointsLayer()
		map.register(pointsLayer)
		const controlText = L('hold Ctrl to zoom', 'ru', 'зажмите Ctrl для зума')
		const twoFingersText = L('use two fingers to drag', 'ru', 'для перетаскивания жмите двумя пальцами')
		map.register(new MapControlHintLayer(controlText, twoFingersText))
		map.resize()

		this.map = map
		this.mapPointsLayer = pointsLayer
	}

	onResize() {
		this.map.resize()
	}

	componentDidMount() {
		addEventListener('resize', this.onResize)
		this.loadData()
		this.setupMap()
	}
	componentWillUnmount() {
		removeEventListener('resize', this.onResize)
	}

	render(props, state) {
		return html`<div class="map-wrap" ref=${this.mapWrapRef}></div>`
	}
}

class NodesSummary extends PureComponent {
	constructor() {
		super()
		this.state = { stats: null }
	}

	loadData() {
		apiReq('GET', `/api/nodes/location_summary`)
			.then(stats => {
				this.setState({ stats })
			})
			.catch(onError)
	}

	componentDidMount() {
		this.loadData()
	}

	render(props, { stats }) {
		const countriesCount = stats && stats.countriesCount

		return html`
			<div class="p-like">
				<table class="underlined wide-padded">
					<thead>
						<tr>
							<td>${L('Country', 'ru', 'Страна')}</td>
							<td>${L('Nodes', 'ru', 'Кол-во')}</td>
						</tr>
					</thead>
					${stats === null
						? zeroes(10).map(
								(_, i) => html`<tr>
									<td class="dim">${L('loading...', 'ru', 'загрузка...')}</td>
									<td class="dim">...</td>
								</tr>`,
						  )
						: stats.countriesTop.map(
								item =>
									html`<tr>
										<td>${item.country}</td>
										<td>${item.count}</td>
									</tr>`,
						  )}
				</table>
				<span class="dim small">
					${L('top 10 countries by nodes number', 'ru', 'топ-10 стран по кол-ву нод')}
				</span>
			</div>
			<p>
				${lang === 'ru'
					? `Ноды запущены как минимум в ${L.n(countriesCount, 'стране', 'странах', 'странах')}.`
					: `Nodes are running in at least ${L.n(countriesCount, 'country', 'countries')}.`}
			</p>
		`
	}
}

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
			<h2>${L('Nodes location', 'ru', 'Расположение нод')}</h2>
			${!intervalIsDefault &&
			html`<p class="warn">
				${lang === 'ru'
					? 'Для местоположений перемотка не работает. Пока.'
					: 'Locations can not rewind. Yet.'}
			</p>`}
			<${NodesLocationMap} />
			<${NodesSummary} />
		`
	}
}
