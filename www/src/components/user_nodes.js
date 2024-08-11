import { useCallback, useEffect, useMemo, useState } from 'preact/hooks'

import { apiReq } from 'src/api'
import { ago, L, lang } from 'src/i18n'
import { onError } from 'src/errors'
import {
	PingModeDescription,
	shortNodeID,
	sortNodes,
	SubnetNeighborsDescription,
	withoutPort,
} from 'src/utils/nodes'
import { bindHandlers } from 'src/utils/elems'
import { memo, PureComponent } from 'src/utils/preact_compat'
import { html } from 'src/utils/htm'
import { Help, HelpLine } from './help'
import { findMeaningfulOctets, isIPv4, resolve, ResolveError } from 'src/utils/dns'
import { isPromise } from 'src/utils/types'

import './user_nodes.css'
import { Fragment } from 'preact'
import { useStorageState } from 'src/utils/store'

/**
 * @typedef {{
 *   id: string,
 *   address: string,
 *   pingMode: 'off'|'dial'|'ping',
 *   lastPingedAt: Date,
 *   lastUpAt: Date,
 *   lastPing: number,
 *   lastPingWasOk: boolean,
 *   isLoading?: boolean
 * }} UserNode
 */

/** @typedef {{foreignNodesCount:number, nodesTotal:number}} NeighborCounts */

/**
 * @typedef {{
 *   as: {
 *     asn: number,
 *     org: string,
 *     type: string,
 *     domain: string,
 *     descr: string,
 *     updatedAt: string,
 *     prefix: string,
 *     ips: string[],
 *   }[],
 *   companies: {
 *     ipFrom: string,
 *     ipTo: string,
 *     name: string,
 *     type: string,
 *     domain: string,
 *     updatedAt: string,
 *     ips: string[],
 * }[],
 * }} IPsInfoResponse
 */

/**
 * @typedef {{
 *   as: IPsInfoResponse['as'] | undefined,
 *   companies: IPsInfoResponse['companies'] | undefined,
 * }} IPInfo
 */

/** @type {UserNode} */
const BLANK_NODE = {
	id: '',
	address: '',
	pingMode: 'off',
	lastPingedAt: new Date(0),
	lastPingWasOk: false,
	lastPing: 0,
	lastUpAt: new Date(0),
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

function DimNA() {
	return html`<span class="dim">${L('N/a', 'ru', 'Н/д')}</span>`
}

/** @param {{counts:NeighborCounts|undefined|Promise<unknown>}} props */
function NodeNeighbors({ counts }) {
	if (isPromise(counts)) return '…'
	if (!counts) return DimNA()
	let status = counts.foreignNodesCount === 0 ? '' : 'warn'
	return html`
		<span class=${status}>
			${counts.foreignNodesCount}
			<span class="dim">/${counts.nodesTotal}</span>
		</span>
	`
}

/** @param {{ipInfo:IPInfo|undefined|Promise<unknown>, ipInfoExpanded:boolean}} props */
function NodeIPInfoCells({ ipInfo, ipInfoExpanded }) {
	const compsPlaceholder = isPromise(ipInfo) ? '…' : !ipInfo?.companies ? DimNA() : undefined
	const asPlaceholder = isPromise(ipInfo) ? '…' : !ipInfo?.as ? DimNA() : undefined
	const comps = isPromise(ipInfo) ? undefined : ipInfo?.companies
	const as = isPromise(ipInfo) ? undefined : ipInfo?.as

	const helpPopupContent = useCallback(() => {
		return html`<${NodeIPInfoFull} ipInfo=${ipInfo} />`
	}, [ipInfo])

	const cells = [
		html`<td class="ip-info ip-company-name ${ipInfoExpanded ? '' : 'compact'}">
			<${HelpLine} contentFunc=${helpPopupContent}>${compsPlaceholder ?? comps?.[0].name ?? '—'}<//>
		</td>`,
	]

	if (ipInfoExpanded) {
		cells.push(
			html`<td class="ip-info ip-as-descr">
					<${HelpLine} contentFunc=${helpPopupContent}>${asPlaceholder ?? as?.[0].descr ?? '—'}<//>
				</td>
				<td class="ip-info ip-as-prefix">
					<${HelpLine} contentFunc=${helpPopupContent}>${asPlaceholder ?? as?.[0].prefix ?? '—'}<//>
				</td>`,
		)
	}

	return html`<${Fragment}>${cells}</${Fragment}>`
}

function NodeIPInfoFull({ ipInfo }) {
	if (isPromise(ipInfo) || !ipInfo) return DimNA()

	return html`<h3>${L('Company', 'ru', 'Компания')}</h3>
		${ipInfo.companies
			? ipInfo.companies.map(
					comp =>
						html`<table class="ip-info-full">
							<tr>
								<th>${L('Name', 'ru', 'Название')}</th>
								<td>${comp.name ?? '—'}</td>
							</tr>
							<tr>
								<th>${L('Domain', 'ru', 'Домен')}</th>
								<td>${comp.domain ?? '—'}</td>
							</tr>
							<tr>
								<th>${L('Type', 'ru', 'Тип')}</th>
								<td>${comp.type ?? '—'}</td>
							</tr>
							<tr>
								<th>${L('IP range', 'ru', 'Диапазон')}</th>
								<td>${comp.ipFrom} – ${comp.ipTo}</td>
							</tr>
							<tr>
								<th>${L('Updated', 'ru', 'Обновлено')}</th>
								<td>${new Date(comp.updatedAt).toLocaleString(lang)}</td>
							</tr>
						</table>`,
			  )
			: html`<p>${DimNA()}</p>`}
		<h3>${L('Autonomous system', 'ru', 'Автономная система')}</h3>
		${ipInfo.as
			? ipInfo.as.map(
					as =>
						html`<table class="ip-info-full">
							<tr>
								<th>ASN</th>
								<td>${as.asn}</td>
							</tr>
							<tr>
								<th>${L('Name', 'ru', 'Название')}</th>
								<td>${as.org ?? '—'}</td>
							</tr>
							<tr>
								<th>${L('Descr', 'ru', 'Описание')}</th>
								<td>${as.descr ?? '—'}</td>
							</tr>
							<tr>
								<th>${L('Domain', 'ru', 'Домен')}</th>
								<td>${as.domain ?? '—'}</td>
							</tr>
							<tr>
								<th>${L('Type', 'ru', 'Тип')}</th>
								<td>${as.type ?? '—'}</td>
							</tr>
							<tr>
								<th>${L('Prefix', 'ru', 'Префикс')}</th>
								<td>${as.prefix}</td>
							</tr>
							<tr>
								<th>${L('Updated', 'ru', 'Обновлено')}</th>
								<td>${new Date(as.updatedAt).toLocaleString(lang)}</td>
							</tr>
						</table>`,
			  )
			: html`<p>${DimNA()}</p>`}`
}

/**
 * @class
 * @typedef UNI_Props
 * @prop {UserNode} node
 * @prop {Date} nodeUpdateTime
 * @prop {(node:UserNode) => void} onChange
 * @prop {(node:UserNode) => void} onRemove
 * @prop {undefined|Error|Promise<unknown>|string} resolvedIP
 * @prop {NeighborCounts|undefined|Promise<unknown>} neighborCounts
 * @prop {IPInfo|undefined|Promise<unknown>} nodeIpInfo
 * @prop {boolean} nodeIpInfoExpanded
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
			${+node.lastPingWasOk
				? html`<p>
						${node.lastUpAt.toLocaleString(lang)}<br />
						${ago(node.lastUpAt)} <span class="dim">${L('ago', 'ru', 'назад')}</span><br />
						${node.lastPing} ${L('ms', 'ru', 'мс')}${' '}
						<span class="dim">${L('response time', 'ru', 'время ответа')}</span>
				  </p>`
				: +node.lastPingedAt < 0
				? html`<p>${L('N/a', 'ru', 'Н/д')}</p>`
				: html`<p>
						${L('Has failed. More info on ', 'ru', 'Провалилось. Подробнее: ')}
						<a href="/ping_my_node">/ping_my_node</a>
				  </p>`}
		`
	}

	/**
	 * @param {UNI_Props} props
	 * @param {{}} state
	 */
	render({ node, nodeUpdateTime, resolvedIP, neighborCounts, nodeIpInfo, nodeIpInfoExpanded }, state) {
		const pingModes = [
			['ping', 'ping'],
			['dial', 'dial'],
			['off', L('off', 'ru', 'выкл')],
		]

		const lastPingedAgo = +nodeUpdateTime - +node.lastPingedAt
		const status =
			node.pingMode === 'off' || lastPingedAgo > 5 * 60 * 1000
				? 'unknown'
				: node.lastPingWasOk
				? 'ok'
				: 'error'

		return html`
			<tr class="node ${node.isLoading ? 'loading' : ''} ${'status-' + status}">
				<td>
					<${HelpLine} contentFunc=${this.onNodeStatusDetails}><div class="node-status"></div><//>
				</td>
				<td>
					<div class="node-id">
						<!-- div, not span: otherwise double click on .full text will select part of .short text -->
						<div class="short">${shortNodeID(node.id)}</div>
						<div class="full">${node.id}</div>
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
										<option value=${name} selected=${name === node.pingMode}>
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
				<${NodeIPInfoCells} ipInfo=${nodeIpInfo} ipInfoExpanded=${nodeIpInfoExpanded} />
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
			.filter(x => x !== '')
			.map(x => {
				let [id, address] = x.split(/\s+/, 2)
				return { ...BLANK_NODE, id, address: address || '' }
			})
			.forEach(this.props.onNodeAdd)
		this.setState({ minimized: true })
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
		<p class="warn">
			${lang === 'ru'
				? 'Обновления автоматически отключатся после месяца оффлайна.'
				: 'Updates will turn off automatically after a month of offline.'}
		</p>
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

export const UserNodesList = memo(function UserNodesList(
	/**
	 * @type {{
	 *   nodes: UserNode[],
	 *   nodesUpdateTime: Date,
	 *   setNode: (node:UserNode) => void,
	 *   delNode: (node:UserNode) => void,
	 * }}
	 */
	{ nodes, nodesUpdateTime, setNode, delNode },
) {
	const [nodeError, setNodeError] = useState(/**@type {string|null}*/ (null))
	const [pendingNodes, setPendingNodes] = useState(/**@type {Record<string, UserNode>}*/ ({}))
	const [neighborCounts, setNeighborCounts] = useState(
		/**@type {Record<string, NeighborCounts|undefined|Promise<unknown>>}*/ ({}),
	)
	const [nodeIpInfos, setNodeIpInfos] = useState(/**@type {Record<string, IPInfo|Promise<unknown>>}*/ ({}))
	const [nodeIpInfoExpanded, setNodeIpInfoExpanded] = //
		useStorageState('nodes_list_ipinfo_expanded', raw => !!raw)

	// special form so we can change outer object (and get reactivity) without copying inner one each time
	const [resolved, setResolved] = useState({
		addrs: /**@type {Record<string,string|Promise<unknown>|Error>}*/ ({}),
	})
	const updateResolved = useCallback(
		(/**@type {string}*/ addr, /**@type {string|Promise<unknown>}*/ res) => {
			const addrs = resolved.addrs
			addrs[addr] = res
			setResolved({ addrs })
		},
		[resolved],
	)
	const finishedResolvedNodeAddrs = useMemo(() => {
		for (const node of nodes) {
			const res = resolved.addrs[withoutPort(node.address)]
			if (typeof res !== 'string' && !(res instanceof Error)) return null
		}
		const ips = nodes //
			.map(x => resolved.addrs[withoutPort(x.address)])
			.filter(x => typeof x === 'string') //skipping errors
		return Array.from(new Set(ips))
	}, [nodes, resolved])

	const sortedNodes = useMemo(() => {
		const woPend = nodes.filter(n => !(n.id in pendingNodes))
		const all = woPend.concat(Object.values(pendingNodes))
		return sortNodes(all)
	}, [nodes, pendingNodes])

	useEffect(() => {
		if (!finishedResolvedNodeAddrs) return

		const abortController = new AbortController()

		const promise = apiReq('POST', '/api/ips_info', {
			data: { ips: finishedResolvedNodeAddrs },
			signal: abortController.signal,
		})
			.then((/**@type {IPsInfoResponse}*/ res) => {
				const asMap = /**@type {Record<string, IPsInfoResponse['as']>}*/ ({})
				const compMap = /**@type {Record<string, IPsInfoResponse['companies']>}*/ ({})

				for (const as of res.as)
					for (const ip of as.ips) //
						(asMap[ip] = asMap[ip] || []).push(as)
				for (const comp of res.companies)
					for (const ip of comp.ips) //
						(compMap[ip] = compMap[ip] || []).push(comp)

				for (const as of Object.values(asMap))
					as.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
				for (const comps of Object.values(compMap))
					comps.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))

				const ipInfos = /**@type {Record<string, IPInfo>}*/ ({})
				for (const node of nodes) {
					let addr = resolved.addrs[withoutPort(node.address)]
					if (typeof addr === 'string') {
						ipInfos[node.id] = { as: asMap[addr], companies: compMap[addr] }
					}
				}
				setNodeIpInfos(ipInfos)
			})
			.catch(onError)

		let ipInfos = /** @type {Record<string, Promise<unknown>>} */ ({})
		for (const node of nodes) ipInfos[node.id] = promise
		setNodeIpInfos(ipInfos)

		return () => abortController.abort()
	}, [nodes, resolved, finishedResolvedNodeAddrs])

	useEffect(() => {
		if (!finishedResolvedNodeAddrs) return

		const abortController = new AbortController()

		let subnets = finishedResolvedNodeAddrs
		let myNodeIds = nodes.map(x => x.id)

		const promise = apiReq('POST', '/api/neighbors', {
			data: { subnets, myNodeIds },
			signal: abortController.signal,
		})
			.then(res => {
				let countsMap = {}
				for (let item of res.counts) countsMap[item.subnet] = item
				let counts = /** @type {Record<string, NeighborCounts|undefined>} */ ({})
				for (const node of nodes) {
					let addr = resolved.addrs[withoutPort(node.address)]
					if (typeof addr === 'string') {
						let subnet = findMeaningfulOctets(addr) + '.0'
						counts[node.id] = countsMap[subnet]
					}
				}
				setNeighborCounts(counts)
			})
			.catch(onError)

		let counts = /** @type {Record<string, Promise<unknown>>} */ ({})
		for (const node of nodes) counts[node.id] = promise
		setNeighborCounts(counts)

		return () => abortController.abort()
	}, [nodes, resolved, finishedResolvedNodeAddrs])

	// should go after /api/neighbors request effect, otherwise request will be sent twice: on this and on next rerender
	useEffect(() => {
		for (const node of nodes) {
			let address = withoutPort(node.address)
			if (address in resolved.addrs) continue

			if (isIPv4(address)) {
				updateResolved(address, address)
				return
			}

			let promise = resolve(address)
				.then(ips => updateResolved(address, ips[0]))
				.catch(err => updateResolved(address, err))
			updateResolved(address, promise)
		}
	}, [nodes, resolved])

	const onIPInfoExpandClick = useCallback(() => {
		setNodeIpInfoExpanded(x => !x)
	}, [])

	const setPendingNode = useCallback(
		(/**@type {UserNode}*/ node) => {
			setPendingNodes({ ...pendingNodes, [node.id]: node })
		},
		[pendingNodes],
	)
	const delNodeFromPending = useCallback(
		(/**@type {UserNode}*/ node) => {
			const np = { ...pendingNodes }
			delete np[node.id]
			setPendingNodes(np)
		},
		[pendingNodes],
	)

	const setNodeInner = useCallback(
		(/**@type {UserNode}*/ node) => {
			setPendingNode({ ...node, isLoading: true })
			setNodeError(null)
			apiReq('POST', '/api/user_nodes', { data: node })
				.then(res => {
					delNodeFromPending(node)
					setNode(node)
				})
				.catch(err => {
					if (err.error === 'NODE_ID_DECODE_ERROR') {
						setNodeError(
							L('Wrong node ID', 'ru', 'Неправильный ID ноды') +
								` "${node.id}": ` +
								err.description,
						)
						delNodeFromPending(node)
					} else onError(err)
				})
		},
		[setPendingNode, delNodeFromPending, setNode, setNodeError, delNodeFromPending],
	)
	const delNodeInner = useCallback(
		(/**@type {UserNode}*/ node) => {
			setPendingNode({ ...node, isLoading: true })
			setNodeError(null)
			apiReq('DELETE', '/api/user_nodes', { data: { id: node.id } })
				.then(res => {
					delNodeFromPending(node)
					delNode(node)
				})
				.catch(onError)
		},
		[setPendingNode, delNodeFromPending, delNode],
	)

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
									<span class="neighbors-title-cell-inner">
										${L('Neighbors', 'ru', 'Соседи')}${' '}
										<${Help} contentFunc=${getNeighborsHelpContent} />
									</span>
								</td>
								<td style="min-width:192px">
									${L('Company', 'ru', 'Компания')}${' '}
									<button class="help" onclick=${onIPInfoExpandClick}>
										${nodeIpInfoExpanded ? '➖\uFE0E' : '➕\uFE0E'}
									</button>
								</td>
								${nodeIpInfoExpanded
									? html`<td>AS</td>
											<td>${L('Prefix', 'ru', 'Префикс')}</td>`
									: null}
								<td></td>
							</tr>
						</thead>
						${sortedNodes.map(
							n =>
								html`
									<${UserNodeItem}
										key=${n.id}
										node=${n}
										nodeUpdateTime=${nodesUpdateTime}
										resolvedIP=${resolved.addrs[withoutPort(n.address)]}
										neighborCounts=${neighborCounts[n.id]}
										nodeIpInfo=${nodeIpInfos[n.id]}
										nodeIpInfoExpanded=${nodeIpInfoExpanded}
										onChange=${setNodeInner}
										onRemove=${delNodeInner}
									/>
								`,
						)}
					</table>
				`}
			</div>
			<${NewUserNodeForm} onNodeAdd=${setNodeInner} />
		</div>
		${nodeError !== null && html`<p class="warn">${nodeError}</p>`}
	`
})
