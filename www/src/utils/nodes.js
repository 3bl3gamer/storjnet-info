import { L } from '../i18n'
import { html } from './htm'

/**
 * @param {{address:string}[]} nodes
 */
export function sortedNodes(nodes) {
	return nodes.sort((a, b) => a.address.localeCompare(b.address))
}

/**
 * @param {string} name
 */
export function shortNodeID(name) {
	return name.slice(0, 4) + '-' + name.slice(-2)
}

export function PingModeDescription() {
	return html`
		<p>
			<b>Dial</b> — ${L(' just try to connect to node', 'ru', ' просто попытаться подключиться к ноде')}
			${' '}(${L('ip:port connection', 'ru', 'коннект на ip:port')} + TLS handshake).
		</p>
		<p>
			<b>Ping</b> —
			${L(
				' connect and send ping (via Storj RPC). Will update',
				'ru',
				' подключиться и отправить пинг (через сторжевый RPC). Обновит',
			)}
			${' '}<code>Last Contact</code>
			${L(' in dashboard', 'ru', ' в дашборде')}.
		</p>
	`
}
