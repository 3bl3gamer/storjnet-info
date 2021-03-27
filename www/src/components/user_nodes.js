import { useCallback } from 'preact/hooks'

import { apiReq } from 'src/api'
import { ago, L, lang, stringifyDuration } from 'src/i18n'
import { onError } from 'src/errors'
import {
	PingModeDescription,
	shortNodeID,
	sortedNodes,
	SubnetNeighborsDescription,
	withoutPort,
} from 'src/utils/nodes'
import { bindHandlers } from 'src/utils/elems'
import { PureComponent } from 'src/utils/preact_compat'
import { html } from 'src/utils/htm'
import { Help, HelpLine } from './help'
import { findMeaningfulOctets, isIPv4, resolve, ResolveError } from 'src/utils/dns'
import { isPromise } from 'src/utils/types'

import './user_nodes.css'

/** @typedef {{
 *   id: string,
 *   address: string,
 *   pingMode: 'off'|'dial'|'ping',
 *   lastPingedAt: Date,
 *   lastUpAt: Date,
 *   lastPing: number,
 *   isLoading?: boolean
 * }} UserNode */
/** @typedef {{foreignNodesCount:number, nodesTotal:number}} NeighborCounts */

/** @type {UserNode} */
const BLANK_NODE = {
	id: '',
	address: '',
	pingMode: 'off',
	lastPingedAt: new Date(0),
	lastUpAt: new Date(0),
	lastPing: 0,
}

/** @param {{ip:string}} props */
function HighlightedSubnet({ ip }) {
	let index = ip.lastIndexOf('.')
	return html`${ip.slice(0, index)}<span class="dim">${ip.slice(index)}</span>`
}

/** @param {{error:Error}} props */
function NodeIPError({ error }) {
	const content = useCallback(
		() =>
			html`<pre>${error instanceof ResolveError ? error.messageLines.join('\n') : error.message}</pre>`,
		[error],
	)
	return html`
		<${HelpLine} contentFunc=${content}>
			<code class="warn">${error.message}</code>
		<//>
	`
}

/** @param {{counts:NeighborCounts|Promise<unknown>}} props */
function NodeNeighbors({ counts }) {
	if (isPromise(counts)) return '…'
	if (!counts) return html`<span class="dim">${L('N/a', 'ru', 'Н/д')}</span>`
	let status = counts.foreignNodesCount === 0 ? '' : 'warn'
	return html`
		<span class=${status}>
			${counts.foreignNodesCount}
			<span class="dim">/${counts.nodesTotal}</span>
		</span>
	`
}

/**
 * @class
 * @typedef UNI_Props
 * @prop {UserNode} node
 * @prop {Date} nodeUpdateTime
 * @prop {(node:UserNode) => void} onChange
 * @prop {(node:UserNode) => void} onRemove
 * @prop {undefined|Error|Promise<unknown>|string} resolvedIP
 * @prop {NeighborCounts|Promise<unknown>} neighborCounts
 * @extends {PureComponent<UNI_Props, {}>}
 */
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
	onNodeStatusDetails() {
		const node = this.props.node
		return html`
			<h3>${L('Last connection attempt', 'ru', 'Последняя попытка подключения')}</h3>
			${+node.lastPingedAt < 0
				? html`<p>${L('N/a', 'ru', 'Н/д')}</p>`
				: html`<p>
						${node.lastPingedAt.toLocaleString(lang)}<br />
						${ago(node.lastPingedAt)} <span class="dim">${L('ago', 'ru', 'назад')}</span>
				  </p>`}
			<h3>${L('Last connection', 'ru', 'Последнее подключение')}</h3>
			${+node.lastUpAt < 0
				? +node.lastPingedAt < 0
					? html`<p>${L('N/a', 'ru', 'Н/д')}</p>`
					: html`<p>
							${L('Has failed. More info on ', 'ru', 'Провалилось. Подробнее: ')}
							<a href="/ping_my_node">/ping_my_node</a>
					  </p>`
				: html`<p>
						${node.lastUpAt.toLocaleString(lang)}<br />
						${ago(node.lastUpAt)} <span class="dim">${L('ago', 'ru', 'назад')}</span><br />
						${node.lastPing} ${L('ms', 'ru', 'мс')}${' '}
						<span class="dim">${L('response time', 'ru', 'время ответа')}</span>
				  </p>`}
		`
	}

	/**
	 * @param {UNI_Props} props
	 * @param {{}} state
	 */
	render({ node, nodeUpdateTime, resolvedIP, neighborCounts }, state) {
		const pingModes = [
			['ping', 'ping'],
			['dial', 'dial'],
			['off', L('off', 'ru', 'выкл')],
		]

		const lastPingedAgo = +nodeUpdateTime - +node.lastPingedAt
		// const lastUpAgo = +nodeUpdateTime - +node.lastUpAt
		const status =
			node.pingMode === 'off' || lastPingedAgo > 5 * 60 * 1000
				? 'unknown'
				: +node.lastUpAt > 0
				? 'ok'
				: 'error'

		return html`
			<tr class="node ${node.isLoading ? 'loading' : ''} ${'status-' + status}">
				<td>
					<${HelpLine} contentFunc=${this.onNodeStatusDetails}><div class="node-status"></div><//>
				</td>
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
						? html`<${NodeIPError} error=${resolvedIP} />`
						: html`<${HighlightedSubnet} ip=${resolvedIP} />`}
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

/**
 * @class
 * @typedef NUNF_Props
 * @prop {(node:UserNode) => void} onNodeAdd
 * @typedef NUNF_State
 * @prop {boolean} minimized
 * @extends {PureComponent<NUNF_Props, NUNF_State>}
 */
class NewUserNodeForm extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		/** @type {NUNF_State} */
		this.state = { minimized: true }
	}
	onSubmit(e) {
		e.preventDefault()
		let data = new FormData(e.target).get('nodes_data') + ''
		data.split('\n')
			.map(x => x.trim())
			.filter(x => x != '')
			.map(x => {
				let [id, address] = x.split(/\s+/, 2)
				return { ...BLANK_NODE, id, address: address || '' }
			})
			.forEach(this.props.onNodeAdd)
	}
	onUnfoldClick() {
		this.setState({ minimized: !this.state.minimized })
	}
	/**
	 * @param {NUNF_Props} props
	 * @param {NUNF_State} state
	 */
	render(props, { minimized }) {
		return html`
			<form class="node-add-form ${minimized ? 'minimized' : ''}" onsubmit=${this.onSubmit}>
				<div class="buttons-wrap">
					<button class="submit-button">Ok</button>
					<button type="button" class="unfold-button" onclick=${this.onUnfoldClick}>
						${minimized ? '➕' : '⮟'}
					</button>
				</div>
				<textarea
					class="nodes-data"
					name="nodes_data"
					placeholder=${L(
						'<node id> <address>\n<node id> <address>\n...',
						'ru',
						'<айди ноды> <адрес>\n<айди ноды> <адрес>\n...',
					)}
				></textarea>
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

/**
 * @class
 * @typedef UNL_Props
 * @prop {UserNode[]} nodes
 * @prop {Date} nodesUpdateTime
 * @prop {(node:UserNode) => void} setNode
 * @prop {(node:UserNode) => void} delNode
 * @typedef UNL_State
 * @prop {string|null} nodeError
 * @prop {Record<string, UserNode>} pendingNodes
 * @prop {Record<string, NeighborCounts|Promise<unknown>>} neighborCounts
 * @extends {PureComponent<UNL_Props, UNL_State>}
 */
export class UserNodesList extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		/** @type {UNL_State} */
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

			let promise = apiReq('POST', '/api/neighbors', { data: { subnets, myNodeIds } }).then(res => {
				let countsMap = {}
				for (let item of res.counts) countsMap[item.subnet] = item
				let neighborCounts = /** @type {Record<string, NeighborCounts>} */ ({})
				for (const node of this.props.nodes) {
					let addr = this.resolvedAddrs[withoutPort(node.address)]
					if (typeof addr === 'string') {
						let subnet = findMeaningfulOctets(addr) + '.0'
						neighborCounts[node.id] = countsMap[subnet]
					}
				}
				this.setState({ neighborCounts })
			})

			let neighborCounts = /** @type {Record<string, Promise<unknown>>} */ ({})
			for (const node of this.props.nodes) neighborCounts[node.id] = promise
			this.setState({ neighborCounts })
		})
	}
	componentDidUpdate(prevProps) {
		for (const node of this.props.nodes) this.resolveIfNeed(node)
	}

	/**
	 * @param {UNL_Props} props
	 * @param {UNL_State} state
	 */
	render({ nodesUpdateTime }, { nodeError, neighborCounts }) {
		let nodes = this.sortedNodes()
		return html`
			<div class="user-nodes-list-with-form">
				<div class="user-nodes-list">
					${nodes.length === 0 && L('No nodes yet', 'ru', 'Нод нет')}
					${nodes.length > 0 &&
					html`
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
										<span style=${{ margin: '0 -21px 0 -8px' }}>
											${L('Neighbors', 'ru', 'Соседи')}${' '}
											<${Help} contentFunc=${getNeighborsHelpContent} />
										</span>
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
											nodeUpdateTime=${nodesUpdateTime}
											resolvedIP=${this.resolvedAddrs[withoutPort(n.address)]}
											neighborCounts=${neighborCounts[n.id]}
											onChange=${this.onNodeChange}
											onRemove=${this.onNodeRemove}
										/>
									`,
							)}
						</table>
					`}
				</div>
				<${NewUserNodeForm} onNodeAdd=${this.onNodeAdd} />
			</div>
			${nodeError !== null && html`<p class="warn">${nodeError}</p>`}
		`
	}
}
