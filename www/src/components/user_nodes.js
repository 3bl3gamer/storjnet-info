import { h } from 'preact'
import { useCallback } from 'preact/hooks'

import { apiReq } from '../api'
import { L, lang } from '../i18n'
import { onError } from '../errors'
import {
	PingModeDescription,
	shortNodeID,
	sortedNodes,
	SubnetNeighborsDescription,
	withoutPort,
} from '../utils/nodes'
import { bindHandlers } from '../utils/elems'
import { PureComponent } from '../utils/preact_compat'
import { html } from '../utils/htm'
import { Help } from './help'
import { findMeaningfulOctets, isIPv4, resolve, ResolveError } from '../utils/dns'
import { isPromise } from '../utils/types'

import './user_nodes.css'

function HighlightedSubnet({ ip }) {
	let index = ip.lastIndexOf('.')
	return html`${ip.slice(0, index)}<span class="dim">${ip.slice(index)}</span>`
}

function NodeIPError({ error }) {
	const content = useCallback(
		() =>
			html`<pre>${error instanceof ResolveError ? error.messageLines.join('\n') : error.message}</pre>`,
		[error],
	)
	return html`
		<span class="warn">
			<${Help} contentFunc=${content} />
			${error.message}
		</span>
	`
}

function NodeNeighbors({ counts }) {
	if (!counts) return html`<span class="dim">${L('N/a', 'ru', 'Н/д')}</span>`
	let status = counts.foreignNodesCount === 0 ? '' : 'warn'
	return html`
		<span class=${status}>
			${counts.foreignNodesCount}
			<span class="dim">/${counts.nodesTotal}</span>
		</span>
	`
}

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

	render({ node, resolvedIP, neighborCounts }, state) {
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
				<td class="node-ip">
					${!resolvedIP || isPromise(resolvedIP)
						? '…'
						: resolvedIP instanceof Error //TODO ResolveError
						? h(NodeIPError, { error: resolvedIP })
						: h(HighlightedSubnet, { ip: resolvedIP })}
				</td>
				<td class="node-neighbors">
					<${NodeNeighbors} counts=${neighborCounts} />
				</td>
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
function getResolvedIPHelpContent() {
	return html`
		<p>
			${lang === 'ru'
				? 'IP и /24-подсеть. Если в качестве адреса ноды указано доменное имя, оно отрезолвится через '
				: 'IP and /24-subnet. If a domain name is used as the node address, it will be resolved via '}
			<a href="https://developers.cloudflare.com/1.1.1.1/dns-over-https/json-format">cloudflare-dns</a>.
		</p>
	`
}

function getNeighborsHelpContent() {
	return html`
		<p>
			<code>foreign</code>/<code>total</code> ${L('where', 'ru', 'где')}<br />
			<code>total</code> —${' '}
			${L('total nodes count in the subnet;', 'ru', 'общее кол-во нод в подсети;')}<br />
			<code>foreign</code> —${' '}
			${lang === 'ru'
				? 'общее кол-во кроме нод с этой страницы (чужие ноды)'
				: 'total count except nodes listed on this page'}.
		</p>
		<${SubnetNeighborsDescription} />
	`
}

export class UserNodesList extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.state = { nodeError: null, pendingNodes: {}, neighborCounts: {} }
		this.resolvedAddrs = {}
	}

	sortedNodes() {
		let nodes = this.props.nodes.filter(n => !(n.id in this.state.pendingNodes))
		nodes = nodes.concat(Object.values(this.state.pendingNodes))
		return sortedNodes(nodes)
	}

	resolveIfNeed(node) {
		let address = withoutPort(node.address)

		if (address in this.resolvedAddrs) return

		if (isIPv4(address)) {
			this.resolvedAddrs[address] = address
			this.forceUpdate()
			return
		}

		let promise = resolve(address)
			.then(ips => (this.resolvedAddrs[address] = ips[0]))
			.catch(err => (this.resolvedAddrs[address] = err))
			.finally(() => this.forceUpdate())
		this.resolvedAddrs[address] = promise
		this.forceUpdate()
	}
	async waitForResolve() {
		while (true) {
			let promises = Object.values(this.resolvedAddrs).filter(isPromise)
			if (promises.length === 0) break
			await Promise.all(promises)
		}
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

	componentDidMount() {
		for (const node of this.props.nodes) this.resolveIfNeed(node)
		this.waitForResolve().then(() => {
			let subnets = this.props.nodes
				.map(x => this.resolvedAddrs[withoutPort(x.address)])
				.filter(x => typeof x === 'string')
			let myNodeIds = this.props.nodes.map(x => x.id)
			apiReq('POST', '/api/neighbors', { data: { subnets, myNodeIds } }).then(res => {
				let countsMap = {}
				for (let item of res.counts) countsMap[item.subnet] = item
				let neighborCounts = {}
				for (const node of this.props.nodes) {
					let addr = this.resolvedAddrs[withoutPort(node.address)]
					if (typeof addr === 'string') {
						let subnet = findMeaningfulOctets(addr) + '.0'
						neighborCounts[node.id] = countsMap[subnet]
					}
				}
				this.setState({ neighborCounts })
			})
		})
	}
	componentDidUpdate(prevProps) {
		for (const node of this.props.nodes) this.resolveIfNeed(node)
	}

	render(props, { nodeError, neighborCounts }) {
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
							<td>
								${L('Subnet', 'ru', 'Подсеть')}${' '}
								<${Help} contentFunc=${getResolvedIPHelpContent} />
							</td>
							<td>
								${L('Neighbors', 'ru', 'Соседи')}${' '}
								<${Help} contentFunc=${getNeighborsHelpContent} />
							</td>
							<td></td>
						</tr>
					</thead>
					${nodes.map(
						n =>
							html`
								<${UserNodeItem}
									key=${n.id}
									node=${n}
									resolvedIP=${this.resolvedAddrs[withoutPort(n.address)]}
									neighborCounts=${neighborCounts[n.id]}
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
