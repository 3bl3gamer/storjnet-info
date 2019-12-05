import './main.css'
import { html, renderIfExists } from './utils'

import './ping_my_node'
import './auth'
import './user_nodes'
import './user_dashboard'

import { PingsChartsList } from './pings_chart'
import { sortedNodes } from './user_nodes'
import { L } from './i18n'

let nodes = []
try {
	nodes = sortedNodes(JSON.parse(document.getElementById('sat_nodes_data').textContent))
} catch (ex) {
	// ¯\_(ツ)_/¯
}

renderIfExists(
	html`
		<h2>
			${L('Satellites', 'ru', 'Сателлиты')}
		</h2>
		<${PingsChartsList} group="sat" nodes=${nodes} />
		<p class="dim small">
			${L(
				'Satellites are pinged from a server near Paris. ' +
					'Narrow red stripes are not a sign of offline: just for some reason a single response was not received.',
				'ru',
				'Сателлиты пингуются из сервера под Парижем. ' +
					'Узкие красные полосы — не признак оффлайна: просто по какой-то причине не вернулся одиночный ответ.',
			)}
		</p>
	`,
	'.sat-nodes',
)
