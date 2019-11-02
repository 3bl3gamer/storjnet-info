import { h } from 'preact'
import { PureComponent, renderIfExists, html, bindHandlers, onError } from './utils'

import './user_nodes.css'

const lang = 'ru'

class UserNodeItem extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
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
			['off', lang == 'ru' ? 'выкл' : 'off'],
		]
		return html`
			<div class="node ${node.isLoading ? 'loading' : ''}">
				<div class="node-id">${node.id}</div>
				<div class="node-params">
					<input
						class="node-address"
						name="address"
						value=${node.address}
						onchange=${this.onChange}
					/>
					<div class="node-ping-mode">
						Проверка аптайма:
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
					<button class="node-remove-button" onclick=${this.onRemoveClick}>✕</button>
				</div>
			</div>
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
				return { id, address, pingMode: 'off' }
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
						placeholder=${lang == 'ru'
							? '<айди ноды> <адрес>\n<айди ноды> <адрес>\n...'
							: '<node id> <address>\n<node id> <address>\n...'}
					></textarea>
				</div>
			</form>
		`
	}
}

class UserNodesList extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		let nodes = []
		try {
			nodes = JSON.parse(document.getElementById('user_nodes_data').textContent)
		} catch (ex) {
			// ¯\_(ツ)_/¯
		}
		this.state = { nodeError: null, nodes: this.sortedNodes(nodes) }
	}

	sortedNodes(nodes) {
		return nodes.sort((a, b) => a.address.localeCompare(b.address))
	}

	setNodeInner(node) {
		let nodes
		let existing = this.state.nodes.find(n => n.id === node.id)
		if (existing) {
			nodes = this.state.nodes.slice()
			nodes[nodes.indexOf(existing)] = node
		} else {
			nodes = this.sortedNodes([...this.state.nodes, node])
		}
		this.setState({ nodes })
	}
	delNodeInner(node) {
		let nodes = this.state.nodes.filter(n => n.id !== node.id)
		this.setState({ nodes })
	}

	setNode(node) {
		this.setNodeInner({ ...node, isLoading: true })
		this.setState({ nodeError: null })
		let body = JSON.stringify(node)
		fetch('/api/user_nodes', { method: 'POST', body })
			.then(r => r.json())
			.then(res => {
				if (res.ok) this.setNodeInner(node)
				else if (res.error == 'NODE_ID_DECODE_ERROR') {
					this.setState({
						nodeError:
							(lang == 'ru' ? 'Неправильный ID ноды' : 'Wrong node ID') +
							` "${node.id}": ` +
							res.description,
					})
					this.delNodeInner(node)
				} else onError(res)
			})
			.catch(onError)
	}
	delNode(node) {
		this.setNodeInner({ ...node, isLoading: true })
		this.setState({ nodeError: null })
		let body = JSON.stringify({ id: node.id })
		fetch('/api/user_nodes', { method: 'DELETE', body })
			.then(r => r.json())
			.then(res => {
				if (res.ok) this.delNodeInner(node)
				else onError(res)
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

	render(props, { nodeError, nodes }) {
		return html`
			<div class="user-nodes-list">
				${nodes.length == 0 && (lang == 'ru' ? 'Нод нет' : 'No nodes yet')}
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
				<${NewUserNodeForm} onNodeAdd=${this.onNodeAdd} />
				${nodeError}
			</div>
		`
	}
}

renderIfExists(h(UserNodesList), '.user-nodes')
