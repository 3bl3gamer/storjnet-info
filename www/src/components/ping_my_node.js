import { apiReq } from 'src/api'
import { isAbortError, onError } from 'src/errors'
import { lang } from 'src/i18n'
import { bindHandlers, getJSONContent } from 'src/utils/elems'
import { html } from 'src/utils/htm'
import { PingModeDescription } from 'src/utils/nodes'
import { PureComponent } from 'src/utils/preact_compat'

import './ping_my_node.css'

/** @typedef {{id:string, address:string}} PingNode */
/** @typedef {{num:number, label:string, quic:boolean}} UsabelSatelliteInfo */

/** @type {UsabelSatelliteInfo[]} */
let usableSatellites = [{ num: 0, label: '???', quic: true }]
try {
	usableSatellites = getJSONContent('usable_satellites')
} catch (ex) {
	// ¯\_(ツ)_/¯
}

/**
 * @class
 * @typedef PMN_State
 * @prop {string} logText
 * @prop {PingNode[]} pingedNodes
 * @prop {PingNode} curNode
 * @prop {boolean} pending
 * @extends {PureComponent<{}, PMN_State>}
 */
export class PingMyNode extends PureComponent {
	constructor() {
		super()
		let pingedNodes = []
		try {
			if (localStorage.pingedNodes) pingedNodes = JSON.parse(localStorage.pingedNodes)
		} catch (ex) {
			// ¯\_(ツ)_/¯
		}
		/** @type {PMN_State} */
		this.state = { pingedNodes, curNode: { id: '', address: '' }, logText: '', pending: false }
		/** @type {AbortController|null} */
		this.pingAbortController = null
		bindHandlers(this)
	}

	addLogLine(msg) {
		this.setState(({ logText }) => ({ logText: logText + '- ' + msg + '\n' }))
	}
	/**
	 * @param {boolean} dialOnly
	 * @param {'tcp'|'quic'} mode
	 * @param {number} satelliteNum
	 * @param {AbortController} abortController
	 * @returns {Promise<{aborted:boolean}>}
	 */
	pingMode(dialOnly, mode, satelliteNum, abortController) {
		let { id, address } = this.state.curNode
		if (id === '' || address === '') return Promise.resolve({ aborted: false })

		this.rememberNode(id, address)
		let log = msg => this.addLogLine(mode.toUpperCase() + ': ' + msg)

		return apiReq('POST', '/api/ping_my_node', {
			data: { id, address, dialOnly, mode, satelliteNum },
			signal: abortController.signal,
		})
			.then(resp => {
				let { dialDuration, pingDuration } = resp
				const ms = seconds => (seconds * 1000).toFixed() + 'ms'
				log(`dialed node in ${ms(dialDuration)}`)
				if (!dialOnly) {
					log(`pinged node in ${ms(pingDuration)}`)
					log(`total: ${ms(pingDuration + dialDuration)}`)
				}
				return { aborted: false }
			})
			.catch(err => {
				switch (err.error) {
					case 'NODE_ID_DECODE_ERROR':
						log('wrong node ID: ' + err.description)
						break
					case 'NODE_DIAL_ERROR':
						log("couldn't connect to node: " + err.description)
						break
					case 'NODE_PING_ERROR':
						log("couldn't ping node: " + err.description)
						break
					default:
						if (isAbortError(err)) {
							log('aborted')
						} else {
							log('O_o ' + JSON.stringify(err))
							onError(err)
						}
				}
				return { aborted: isAbortError(err) }
			})
	}
	/**
	 * @param {boolean} dialOnly
	 */
	async pingAllModes(dialOnly) {
		this.setState({ pending: true, logText: '' })

		if (this.pingAbortController !== null) this.pingAbortController.abort()
		let abortController = (this.pingAbortController = new AbortController())

		this.addLogLine('started')
		this.addLogLine('')

		try {
			for (const sat of usableSatellites) {
				this.addLogLine('via: ' + sat.label + (sat.quic ? '' : ' (proxy, no QUIC)'))

				/** @type {Promise<{aborted:boolean}>[]} */
				let promises = []
				promises.push(this.pingMode(dialOnly, 'tcp', sat.num, abortController))
				if (sat.quic) promises.push(this.pingMode(dialOnly, 'quic', sat.num, abortController))
				await Promise.all(promises)

				this.addLogLine('')
			}
		} finally {
			this.setState({ pending: false })
			this.pingAbortController = null
		}

		this.addLogLine('done.')
	}
	rememberNode(id, address) {
		let nodes = this.state.pingedNodes
		if (!nodes.find(n => n.id === id && n.address === address)) {
			nodes = nodes.slice()
			nodes.push({ id, address })
			this.setState({ pingedNodes: nodes })
		}
	}
	forgetNode(id, address) {
		this.setState({
			pingedNodes: this.state.pingedNodes.filter(n => n.id !== id || n.address !== address),
		})
	}

	onNodeClick(e) {
		let elem = e.target.closest('.item')
		this.setState({
			curNode: { id: elem.dataset.id, address: elem.dataset.address },
		})
	}
	onNodeRemoveClick(e) {
		let elem = e.target.closest('.item')
		this.forgetNode(elem.dataset.id, elem.dataset.address)
	}
	onDialClick() {
		this.pingAllModes(true)
	}
	onPingClick() {
		this.pingAllModes(false)
	}
	onCurNodeIDUpdate(e) {
		this.setState({ curNode: { ...this.state.curNode, id: e.target.value.trim() } })
	}
	onCurNodeAddressUpdate(e) {
		this.setState({ curNode: { ...this.state.curNode, address: e.target.value.trim() } })
	}

	componentDidUpdate(prevProps, prevState) {
		localStorage.pingedNodes = JSON.stringify(this.state.pingedNodes)
	}

	/**
	 * @param {{}} props
	 * @param {PMN_State} state
	 */
	render(props, { pingedNodes, curNode, logText, pending }) {
		return html`
			<div class="remembered-nodes-list">
				${pingedNodes.map(
					node => html`
						<div class="item" data-id=${node.id} data-address=${node.address}>
							<a class="node" href="javascript:void(0)" onclick=${this.onNodeClick}>
								<div class="node-id">${node.id}</div>
								<div class="node-address">${node.address}</div>
							</a>
							<button class="remove" onclick=${this.onNodeRemoveClick}>✕</button>
						</div>
					`,
				)}
			</div>
			<${PingModeDescription} />
			<p>
				${lang === 'ru' ? 'Будут проверены и TCP, и ' : 'Will try both TCP and '}
				<a href="https://forum.storj.io/t/experimenting-with-udp-based-protocols/11545">QUIC</a>.
			</p>
			<form class="node-ping-form">
				<input
					class="node-id-input"
					placeholder="Node ID"
					value=${curNode.id}
					onchange=${this.onCurNodeIDUpdate}
				/>
				<input
					class="node-address-input"
					placeholder="1.2.3.4:28967"
					value=${curNode.address}
					onchange=${this.onCurNodeAddressUpdate}
				/>
				<input class="node-dial-button" type="button" value="Dial" onclick=${this.onDialClick} />
				<input class="node-ping-button" type="button" value="Ping" onclick=${this.onPingClick} />
			</form>
			<pre class="log-box ${pending ? 'pending' : ''}">${logText}</pre>
		`
	}
}
