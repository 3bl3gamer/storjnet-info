import { apiReq } from '../api'
import { onError } from '../errors'
import { L, lang, pluralize } from '../i18n'
import { findMeaningfulOctets, resolveSubnetOrNull } from '../utils/dns'
import { bindHandlers } from '../utils/elems'
import { html } from '../utils/htm'
import { SubnetNeighborsDescription } from '../utils/nodes'
import { PureComponent } from '../utils/preact_compat'

function logLine(msg) {
	return '- ' + msg + '\n'
}

function unexpectedErrText(err) {
	return L(`Something went wrong`, 'ru', 'Что-то пошло не так') + ': ' + err
}

export class SearchNeighbors extends PureComponent {
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
		return resolveSubnetOrNull(name, this.addLogLines.bind(this))
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
			<div class="p-like">
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
			</div>
			<p>
				${isLoading
					? L('Loading…', 'ru', 'Загрузка…')
					: count == null
					? '\xA0' //nbsp
					: lang === 'ru'
					? html`В подсети ${pluralize(count, 'нашлась', 'нашлось', 'нашлось')}${' '}
							<b>${L.n(count, 'нода', 'ноды', 'нод')}</b>${' '}
							<span class="dim">
								${pluralize(count, 'активная', 'активные', 'активных')} за последние 24 часа
							</span>`
					: html`<b>${L.n(count, 'node', 'nodes')}</b> ${pluralize(count, 'was', 'were')} found in
							the subnet${' '} <span class="dim"> reachable within the last 24 hours </span>`}
			</p>
			${logText && html`<pre>${logText}</pre>`}
			<br />
			<${SubnetNeighborsDescription} classes="dim" />
		`
	}
}
