import { h } from 'preact'
import { PureComponent, renderIfExists, html, bindHandlers } from './utils'

import './ping_my_node.css'

const lang = 'ru'

class PingMyNodeModule extends PureComponent {
	constructor() {
		super()
		let pingedNodes = []
		try {
			if (localStorage.pingedNodes) pingedNodes = JSON.parse(localStorage.pingedNodes)
		} catch (ex) {
			// ¯\_(ツ)_/¯
		}
		this.state = { pingedNodes, curNode: { id: '', address: '' }, logText: '' }
		this.pingAbortController = null
		bindHandlers(this)
	}

	logLine(msg) {
		return '- ' + msg + '\n'
	}
	ping(dialOnly) {
		let { id, address } = this.state.curNode
		if (id == '' || address == '') return

		if (this.pingAbortController !== null) this.pingAbortController.abort()
		this.pingAbortController = new AbortController()

		this.rememberNode(id, address)
		this.setState({ logText: this.logLine('started') })

		fetch('/api/ping_my_node', {
			method: 'POST',
			body: JSON.stringify({ id, address, dialOnly }),
			signal: this.pingAbortController.signal,
		})
			.then(r => r.json())
			.then(resp => {
				let logText = this.state.logText
				let log = msg => (logText += this.logLine(msg))

				this.pingAbortController = null
				if (resp.ok) {
					let { dialDuration, pingDuration } = resp.result
					const ms = seconds => (seconds * 1000).toFixed() + 'ms'
					log(`dialed node in ${ms(dialDuration)}`)
					if (!dialOnly) {
						log(`pinged node in ${ms(pingDuration)}`)
						log(`total: ${ms(pingDuration + dialDuration)}`)
					}
				} else {
					switch (resp.error) {
						case 'NODE_ID_DECODE_ERROR':
							log('wrong node ID: ' + resp.description)
							break
						case 'NODE_DIAL_ERROR':
							log("couldn't connect to node: " + resp.description)
							break
						case 'NODE_PING_ERROR':
							log("couldn't ping node: " + resp.description)
							break
						default:
							log('O_o ' + JSON.stringify(resp))
					}
				}
				this.setState({ logText })
			})
	}
	rememberNode(id, address) {
		let nodes = this.state.pingedNodes
		if (!nodes.find(n => n.id == id && n.address == address)) {
			nodes = nodes.slice()
			nodes.push({ id, address })
			this.setState({ pingedNodes: nodes })
		}
	}
	forgetNode(id, address) {
		this.setState({
			pingedNodes: this.state.pingedNodes.filter(n => n.id != id || n.address != address),
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
		this.ping(true)
	}
	onPingClick() {
		this.ping(false)
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

	render(props, { pingedNodes, curNode, logText }) {
		return html`
			<div class="remembered-nodes-list">
				${pingedNodes.map(
					node => html`
						<div class="item" data-id=${node.id} data-address=${node.address}>
							<a class="node" href="javascript:void(0)" onclick=${this.onNodeClick}>
								<span class="node-id">${node.id}</span>
								<span class="node-address">${node.address}</span>
							</a>
							<button class="remove" onclick=${this.onNodeRemoveClick}>✕</button>
						</div>
					`,
				)}
			</div>
			<p>
				<b>Dial</b> —
				${lang == 'ru'
					? ' просто попытаться подключиться к ноде.'
					: ' just try to connect to node.'}
			</p>
			<p>
				<b>Ping</b> —
				${lang == 'ru'
					? ' подключиться и отправить пинг. Обновит'
					: ' connect and send ping. Will update'} <code>Last Contact</code>.
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
				<input
					class="node-dial-button"
					type="button"
					value="Dial"
					onclick=${this.onDialClick}
				/>
				<input
					class="node-ping-button"
					type="button"
					value="Ping"
					onclick=${this.onPingClick}
				/>
			</form>
			<pre class="log-box">${logText}</pre>
		`
	}
}

renderIfExists(h(PingMyNodeModule), '.module.ping-my-node')
