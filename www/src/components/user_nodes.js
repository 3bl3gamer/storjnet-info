import { apiReq } from '../api'
import { L } from '../i18n'
import { onError } from '../errors'
import { PingModeDescription, shortNodeID, sortedNodes } from '../utils/nodes'
import { bindHandlers } from '../utils/elems'
import { PureComponent } from '../utils/preact_compat'
import { html } from '../utils/htm'

import './user_nodes.css'
import { Help } from './help'
import { h } from 'preact'

class UserNodeItem extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.state = {}
	}
	onChange(e) {
		let changed = { ...this.props.node }
		changed[e.target.name] = e.target.value
		this.props.onChange(changed)
	}
	onRemoveClick(e) {
		this.props.onRemove(this.props.node)
	}
	render({ node }, state) {
		const pingModes = [
			['ping', 'ping'],
			['dial', 'dial'],
			['off', L('off', 'ru', 'выкл')],
		]
		return html`
			<tr class="node ${node.isLoading ? 'loading' : ''}">
				<td><div class="node-status"></div></td>
				<td>
					<div class="node-id">
						<span class="short">${shortNodeID(node.id)}</span>
						<span class="full">${node.id}</span>
					</div>
				</td>
				<td>
					<input
						class="node-address"
						name="address"
						value=${node.address}
						onchange=${this.onChange}
					/>
				</td>
				<td>
					<div class="node-ping-mode">
						<!-- ${L('Uptime check', 'ru', 'Проверка аптайма')}: -->
						<select name="pingMode" onchange=${this.onChange}>
							${pingModes.map(
								([name, label]) =>
									html`
										<option value=${name} selected=${name == node.pingMode}>
											${label}
										</option>
									`,
							)}
						</select>
					</div>
				</td>
				<!-- <td>1<span class="dim">/2</span></td> -->
				<td>
					<button class="node-remove-button" onclick=${this.onRemoveClick}>✕</button>
				</td>
			</tr>
		`
	}
}

class NewUserNodeForm extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.state = { minimized: true }
		this.ignoreSubmitsUntil = 0
	}
	onSubmit(e) {
		e.preventDefault()
		if (Date.now() < this.ignoreSubmitsUntil) return

		new FormData(e.target)
			.get('nodes_data')
			.split('\n')
			.map(x => x.trim())
			.filter(x => x != '')
			.map(x => {
				let [id, address] = x.split(/\s+/, 2)
				return { id, address: address || '', pingMode: 'off' }
			})
			.forEach(this.props.onNodeAdd)
	}
	onEnterForm(e) {
		this.ignoreSubmitsUntil = Date.now() + 100
	}
	render(props, { minimized }) {
		return html`
			<form
				class="node-add-form ${minimized ? 'minimized' : ''}"
				onmouseenter=${this.onEnterForm}
				onsubmit=${this.onSubmit}
			>
				<button type="button" class="unfold-button">➕</button>
				<div class="unfolding-elems">
					<button class="node-add-button">➕</button>
					<textarea
						class="nodes-data"
						name="nodes_data"
						placeholder=${L(
							'<node id> <address>\n<node id> <address>\n...',
							'ru',
							'<айди ноды> <адрес>\n<айди ноды> <адрес>\n...',
						)}
					></textarea>
				</div>
			</form>
		`
	}
}

function getPingModeHelpContent() {
	return html`
		<p>${L('Availability check (once a minute)', 'ru', 'Проверка доступности (раз в минуту).')}</p>
		<${PingModeDescription} />
	`
}

export class UserNodesList extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.state = { nodeError: null, pendingNodes: {} }
	}

	sortedNodes() {
		let nodes = this.props.nodes.filter(n => !(n.id in this.state.pendingNodes))
		nodes = nodes.concat(Object.values(this.state.pendingNodes))
		return sortedNodes(nodes)
	}

	setPendingNode(node) {
		let pendingNodes = { ...this.state.pendingNodes, [node.id]: node }
		this.setState({ pendingNodes })
	}
	delNodeInner(node) {
		let pendingNodes = { ...this.state.pendingNodes }
		delete pendingNodes[node.id]
		this.setState({ pendingNodes })
	}
	applySetNode(node) {
		this.delNodeInner(node)
		this.props.setNode(node)
	}
	applyDelNode(node) {
		this.delNodeInner(node)
		this.props.delNode(node)
	}

	setNode(node) {
		this.setPendingNode({ ...node, isLoading: true })
		this.setState({ nodeError: null })
		apiReq('POST', '/api/user_nodes', { data: node })
			.then(res => {
				this.applySetNode(node)
			})
			.catch(err => {
				if (err.error == 'NODE_ID_DECODE_ERROR') {
					this.setState({
						nodeError:
							L('Wrong node ID', 'ru', 'Неправильный ID ноды') +
							` "${node.id}": ` +
							err.description,
					})
					this.delNodeInner(node)
				} else onError(err)
			})
	}
	delNode(node) {
		this.setPendingNode({ ...node, isLoading: true })
		this.setState({ nodeError: null })
		apiReq('DELETE', '/api/user_nodes', { data: { id: node.id } })
			.then(res => {
				this.applyDelNode(node)
			})
			.catch(onError)
	}

	onNodeAdd(node) {
		this.setNode(node)
	}
	onNodeChange(node) {
		this.setNode(node)
	}
	onNodeRemove(node) {
		this.delNode(node)
	}

	render(props, { nodeError }) {
		let nodes = this.sortedNodes()
		return html`
			<div class="user-nodes-list">
				${nodes.length == 0 && L('No nodes yet', 'ru', 'Нод нет')}
				<table class="user-nodes-table">
					<thead>
						<tr>
							<td></td>
							<td>${L('Node ID', 'ru', 'ID ноды')}</td>
							<td>${L('Address', 'ru', 'Адрес')}</td>
							<td>
								${L('Test', 'ru', 'Тест')}${' '}
								<${Help} contentFunc=${getPingModeHelpContent} />
							</td>
							<!-- <td>${L('Neighbors', 'ru', 'Соседи')} <${Help} textFunc=${() =>
								'asd'} /></td> -->
							<td></td>
						</tr>
					</thead>
					${nodes.map(
						n =>
							html`
								<${UserNodeItem}
									key=${n.id}
									node=${n}
									onChange=${this.onNodeChange}
									onRemove=${this.onNodeRemove}
								/>
							`,
					)}
				</table>
				<${NewUserNodeForm} onNodeAdd=${this.onNodeAdd} />
				${nodeError}
			</div>
		`
	}
}
