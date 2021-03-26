import createStore from 'unistore'
import { Provider, connect as connect_ } from 'unistore/src/integrations/preact'

import { html } from 'src/utils/htm'
import { sortedNodes } from 'src/utils/nodes'
import { UserNodesList } from './user_nodes'
import { PingsChartsList } from './pings_chart'
import { getJSONContent } from 'src/utils/elems'

import './user_dashboard.css'

// fixig types. could import it just from unistore/preact, but will have to add rollup commonjs plugin
const connect = /** @type {import('unistore/preact').connect} */ (/**@type {*}*/ (connect_))

let nodes = []
try {
	nodes = sortedNodes(getJSONContent('user_nodes_data'))
} catch (ex) {
	// ¯\_(ツ)_/¯
}
let store = createStore({ nodes })

let nodesActions = {
	setNode(state, node) {
		let nodes
		let existing = state.nodes.find(n => n.id === node.id)
		if (existing) {
			nodes = state.nodes.slice()
			nodes[nodes.indexOf(existing)] = node
		} else {
			nodes = sortedNodes([...state.nodes, node])
		}
		return { nodes }
	},
	delNode(state, node) {
		let nodes = state.nodes.filter(n => n.id !== node.id)
		return { nodes }
	},
}

let UserNodesListS = connect('nodes', nodesActions)(UserNodesList)
let PingsChartsListS = connect('nodes', nodesActions)(PingsChartsList)

export function UserDashboardNodes() {
	return html`
		<${Provider} store=${store}>
			<${UserNodesListS} />
		<//>
	`
}

export function UserDashboardPings() {
	return html`
		<${Provider} store=${store}>
			<${PingsChartsListS} group="my" />
		<//>
	`
}
