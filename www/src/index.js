import './main.css'
import { html, renderIfExists } from './utils'

import './ping_my_node'
import './auth'
import './user_nodes'
import './user_dashboard'

import { PingsChartsList } from './pings_chart'
import { sortedNodes } from './user_nodes'

const lang = 'ru'

let nodes = []
try {
	nodes = sortedNodes(JSON.parse(document.getElementById('sat_nodes_data').textContent))
} catch (ex) {
	// ¯\_(ツ)_/¯
}

renderIfExists(
	html`
		<h2>
			${lang == 'ru' ? 'Сателлиты' : 'Satellites'}
		</h2>
		<${PingsChartsList} group="sat" nodes=${nodes} />
		<p class="dim small">
			${lang == 'ru'
				? 'Сателлиты пингуются из из сервера под Парижем. Узкие красные полосы — не признак оффлайна: просто по какой-то причине не вернулся одиночный пинг.'
				: 'Satellites are pinged from a server near Paris. Narrow red stripes are not a sign of offline: just for some reason a single ping response was not received.'}
		</p>
	`,
	'.sat-nodes',
)
