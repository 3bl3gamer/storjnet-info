import createStore from 'unistore'

import { sortNodes } from 'src/utils/nodes'
import { UserNodesList } from './user_nodes'
import { PingsChartsList } from './pings_chart'
import { getJSONContent } from 'src/utils/elems'

import './user_dashboard.css'
import { h } from 'preact'
import { connectAndWrap } from 'src/utils/store'

function convertFromJSON(node) {
	node.lastPingedAt = new Date(node.lastPingedAt)
	node.lastUpAt = new Date(node.lastUpAt)
	return node
}

let storeData = { nodes: [], nodesUpdateTime: new Date() }
try {
	let data = getJSONContent('user_nodes_data')
	storeData.nodes = sortNodes(data.nodes.map(convertFromJSON))
	storeData.nodesUpdateTime = new Date(data.updateTime)
} catch (ex) {
	// ¯\_(ツ)_/¯
}
let store = createStore(storeData)

let nodesActions = {
	setNode(state, node) {
		let nodes
		let existing = state.nodes.find(n => n.id === node.id)
		if (existing) {
			nodes = state.nodes.slice()
			nodes[nodes.indexOf(existing)] = node
		} else {
			nodes = sortNodes([...state.nodes, node])
		}
		return { nodes }
	},
	delNode(state, node) {
		let nodes = state.nodes.filter(n => n.id !== node.id)
		return { nodes }
	},
}

export const UserDashboardNodes = connectAndWrap(
	UserNodesList,
	store,
	['nodes', 'nodesUpdateTime'],
	nodesActions,
)
export const UserDashboardPings = connectAndWrap(
	props => h(PingsChartsList, { ...props, group: 'my' }),
	store,
	'nodes',
	nodesActions,
)
