import createStore from 'unistore'

import { sortedNodes } from 'src/utils/nodes'
import { UserNodesList } from './user_nodes'
import { PingsChartsList } from './pings_chart'
import { getJSONContent } from 'src/utils/elems'

import './user_dashboard.css'
import { h } from 'preact'
import { connectAndWrap } from 'src/utils/store'

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

export const UserDashboardNodes = connectAndWrap(UserNodesList, store, 'nodes', nodesActions)
export const UserDashboardPings = connectAndWrap(
	props => h(PingsChartsList, { ...props, group: 'my' }),
	store,
	'nodes',
	nodesActions,
)
