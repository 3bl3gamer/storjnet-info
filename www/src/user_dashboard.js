import { html, renderIfExists } from './utils'
import { Provider, connect } from 'unistore/src/integrations/preact'
import createStore from 'unistore'
import { sortedNodes, UserNodesList } from './user_nodes'
import { PingsChartsList } from './pings_chart'

let nodes = []
try {
	nodes = sortedNodes(JSON.parse(document.getElementById('user_nodes_data').textContent))
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

renderIfExists(
	html`
		<${Provider} store=${store}>
			<div>
				<${UserNodesListS} />
				<${PingsChartsListS} group="my" />
			</div>
		<//>
	`,
	'.user-dashboard',
)
