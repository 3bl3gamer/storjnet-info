import { L, lang } from 'src/i18n'
import { html } from './htm'

/**
 * @template {{address:string}} T
 * @param {T[]} nodes
 */
export function sortNodes(nodes) {
	return nodes.sort((a, b) => a.address.localeCompare(b.address))
}

/**
 * @param {string} name
 */
export function shortNodeID(name) {
	return name.slice(0, 4) + '-' + name.slice(-2)
}

/**
 * @param {string} addr
 */
export function withoutPort(addr) {
	let index = addr.lastIndexOf(':')
	return index === -1 ? addr : addr.slice(0, index)
}

export function PingModeDescription() {
	return html`
		<p>
			<b>Dial</b> — ${L(' just try to connect to node', 'ru', ' просто попытаться подключиться к ноде')}
			${' '}(${L('ip:port connection', 'ru', 'коннект на ip:port')} + TLS handshake).
		</p>
		<p>
			<b>Ping</b> —
			${lang === 'ru'
				? ' подключиться и отправить пинг (через сторжевый RPC). Обновит'
				: ' connect and send ping (via Storj RPC). Will update'}
			${' '}<code>Last Contact</code>
			${L(' in dashboard', 'ru', ' в дашборде')}.
		</p>
	`
}

export function SubnetNeighborsDescription() {
	return html`
		<p>
			${L('Since traffic is ', 'ru', 'Т.к. трафик ')}
			<a href="https://forum.storj.io/t/storage-nodes-on-the-same-subnet-different-public-ips/3476/3">
				${lang === 'ru'
					? 'делится между всеми нодами в /24-подсети'
					: 'divided between all nodes in /24-subnet'}
			</a>
			${lang === 'ru'
				? ', лучше разносить ноды по разным подсетям или хотя бы знать, что рядом кто-то делит трафик.'
				: ", it's good to keep nodes on different subnets or at least know if someone is sharing traffic."}
		</p>
		<p>
			${lang === 'ru'
				? `Некоторые ноды (особенно новые) могут не учитываться. ` +
				  `Если есть сомнения, лучше проверить свою подсеть вручную (например Nmap'ом).`
				: 'Some nodes (especially new ones) may not be found. ' +
				  'If in doubt, better check your subnet manually (e.g. with Nmap).'}
		</p>
	`
}
