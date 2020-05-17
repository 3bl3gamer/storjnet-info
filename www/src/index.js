import './main.css'
import { html, renderIfExists } from './utils'

import './ping_my_node'
import './auth'
import './user_nodes'
import './user_dashboard'

import { L } from './i18n'
import { PingsChartsList } from './pings_chart'
import { sortedNodes } from './user_nodes'
import { StorjTxSummary } from './storj_tx_summary'
import { NodesLocationSummary } from './nodes_location_summary'

let nodes = []
try {
	nodes = sortedNodes(JSON.parse(document.getElementById('sat_nodes_data').textContent))
} catch (ex) {
	// ¯\_(ツ)_/¯
}

renderIfExists(
	html`
		<h2>${L('Satellites', 'ru', 'Сателлиты')}</h2>
		<${PingsChartsList} group="sat" nodes=${nodes} />
		<p class="dim small">
			${L(
				'Once a minute a connection is established with the satellites from a server near Paris, ' +
					'elapsed time is saved. Timeous is 2 s. ' +
					'Narrow red stripes are not a sign of offline: just for some reason a single response was not received.',
				'ru',
				'Раз в минуту с сателлитами устанавливается соединение из сервера под Парижем, ' +
					'затраченное время сохраняется. Таймаут — 2 с. ' +
					'Узкие красные полосы — не 100%-признак оффлайна: просто по какой-то причине не вернулся одиночный ответ.',
			)}
		</p>
	`,
	'.sat-nodes',
)

renderIfExists(
	html`
		<h2>${L('Payouts', 'ru', 'Выплаты')}</h2>
		<${StorjTxSummary} />
	`,
	'.storj-tx-summary',
)

renderIfExists(
	html`
		<h2>${L('Nodes location', 'ru', 'Расположение нод')}</h2>
		<${NodesLocationSummary} />
	`,
	'.nodes-location-summary',
)
