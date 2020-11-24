import { h } from 'preact'
import { apiReq } from './api'
import { L, lang, pluralize } from './i18n'
import { bindHandlers, html, onError, PureComponent, renderIfExists } from './utils'

function findMeaningfulOctets(value) {
	const m = value.trim().match(/^(\d+\.\d+\.\d+)(?:\.\d+(?:\/24)?)?$/)
	if (m === null) return null
	if (m[1].split('.').some(x => parseInt(x) > 255)) return null
	return m[1]
}

// https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml#dns-parameters-6
const RESOLVE_STATUS_NAMES_MAP = {
	0: 'No Error',
	1: 'Format Error',
	2: 'Server Failure',
	3: 'Non-Existent Domain',
	4: 'Not Implemented',
	5: 'Query Refused',
	6: 'Name Exists when it should not',
	7: 'RR Set Exists when it should not',
	8: 'RR Set that should exist does not',
	9: 'Server Not Authoritative for zone',
	9: 'Not Authorized',
	10: 'Name not contained in zone',
	11: 'DSO-TYPE Not Implemented',
	// 12-15: Unassigned
	16: 'Bad OPT Version',
	16: 'TSIG Signature Failure',
	17: 'Key not recognized',
	18: 'Signature out of time window',
	19: 'Bad TKEY Mode',
	20: 'Duplicate key name',
	21: 'Algorithm not supported',
	22: 'Bad Truncation',
	23: 'Bad/missing Server Cookie',
	// 24-3840: Unassigned
	// 3841-4095: Reserved for Private Use
	// 4096-65534: Unassigned
	// 65535: Reserved, can be allocated by Standards Action
}

function logLine(msg) {
	return '- ' + msg + '\n'
}

function unexpectedErrText(err) {
	return L(`Something went wrong`, 'ru', 'Что-то пошло не так') + ': ' + err
}

class SearchNeighbors extends PureComponent {
	constructor() {
		super()
		this.state = { isLoading: false, logText: '', count: null }
		this.abortController = null
		bindHandlers(this)
	}

	addLogLines(...lines) {
		this.setState({ logText: this.state.logText + lines.map(logLine).join('') })
	}

	resolve(name) {
		const nameEnc = encodeURIComponent(name)
		const ct = encodeURIComponent('application/dns-json')
		// https://developers.cloudflare.com/1.1.1.1/dns-over-https/json-format
		return fetch(`https://cloudflare-dns.com/dns-query?name=${nameEnc}&type=A&ct=${ct}`)
			.then(r => r.json())
			.then(response => {
				if (response.Status !== 0) {
					const status = RESOLVE_STATUS_NAMES_MAP[response.Status] || 'Unknown Error'
					this.addLogLines(
						`Error: Can not resolve ${name}: ${status}`,
						'Full response: ' + JSON.stringify(response, null, '  '),
					)
					return null
				}
				const ip = (response.Answer && response.Answer[0] && response.Answer[0].data) + ''
				const subnet = findMeaningfulOctets(ip)
				if (subnet === null) {
					this.addLogLines(
						`Error: Expected IPv4-addres in Answer[0].data, got ${ip}`,
						'Full response: ' + JSON.stringify(response, null, '  '),
					)
					return null
				}
				this.addLogLines(
					`resolved to [${response.Answer.map(x => x.data).join(', ')}], using ${subnet}.0`,
				)
				return subnet
			})
			.catch(err => {
				onError(err)
				this.addLogLines(unexpectedErrText(err))
				return null
			})
	}

	async onSubmit(e) {
		e.preventDefault()
		const address = (new FormData(e.target).get('address') || '').trim()
		if (address === '') return

		this.setState({ isLoading: true, logText: '', count: null })
		await Promise.resolve() //костыль, чтоб в this.state попал пустой logText

		let subnet = findMeaningfulOctets(address)
		if (subnet === null) {
			this.addLogLines(
				`${address} ` +
					L(`doesn't look like IP, `, 'ru', 'не похоже на IP, ') +
					L(`trying to resolve via`, 'ru', 'попытаемся отрезолвить через') +
					' cloudflare-dns.com...',
			)
			subnet = await this.resolve(address)
		}
		if (subnet === null) {
			this.setState({ isLoading: false })
			return
		}

		try {
			const resp = await apiReq('GET', `/api/neighbors/${subnet}.0`)
			this.setState({ count: resp.count })
		} catch (err) {
			if (err.error === 'WRONG_SUBNET_FORMAT') {
				this.addLogLines(L('wrong format', 'ru', 'неправильный формат'))
			} else {
				onError(err)
				this.addLogLines(unexpectedErrText(err))
			}
		} finally {
			this.setState({ isLoading: false })
		}
	}

	render({}, { isLoading, logText, count }) {
		return html`
			<p>
				<form class="search-neighbors-form" onSubmit=${this.onSubmit}>
					<input
						class="address-input"
						placeholder=${L('IP/subnet/DNS-name', 'ru', 'IP/подсеть/DNS-имя')}
						name="address"
						autofocus
						required
					/>${' '}
					<input type="submit" value="OK" disabled=${isLoading} />
				</form>
			</p>
			<p>
				${
					isLoading
						? L('Loading…', 'ru', 'Загрузка…')
						: count == null
						? '\xA0' //nbsp
						: lang === 'ru'
						? html`В подсети ${pluralize(count, 'нашлась', 'нашлось', 'нашлось')}${' '}
								<b>${L.n(count, 'нода', 'ноды', 'нод')}</b>${' '}
								<span class="dim">
									${pluralize(count, 'активная', 'активные', 'активных')} за последние 24
									часа
								</span>`
						: html`<b>${L.n(count, 'node', 'nodes')}</b> ${pluralize(count, 'was', 'were')} found
								in the subnet${' '}
								<span class="dim">
									reachable within the last 24 hours
								</span>`
				}
			</p>
			${logText && html`<pre>${logText}</pre>`}
			<p class="dim">
				${
					lang === 'ru'
						? `Некоторые ноды (особенно новые) могут не учитываться. ` +
						  `Если есть сомнения, лучше проверить свою подсеть вручную (например Nmap'ом).`
						: 'Some nodes (especially new ones) may not be found. ' +
						  'If in doubt, better check your subnet manually (e.g. with Nmap).'
				}
			</p>
		`
	}
}

renderIfExists(h(SearchNeighbors), '.module.search-neighbors')
