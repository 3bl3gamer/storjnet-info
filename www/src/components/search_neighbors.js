import { apiReq, apiReqIPsSanctions } from 'src/api'
import { onError } from 'src/errors'
import { L, lang, pluralize } from 'src/i18n'
import { findMeaningfulOctets, isIPv4, resolveSubnetOrNull } from 'src/utils/dns'
import { bindHandlers, NBSP } from 'src/utils/elems'
import { html } from 'src/utils/htm'
import { NodeSanctionDescr, SubnetNeighborsDescription } from 'src/utils/nodes'
import { PureComponent } from 'src/utils/preact_compat'

function logLine(msg) {
	return '- ' + msg + '\n'
}

function unexpectedErrText(err) {
	return L(`Something went wrong`, 'ru', 'Что-то пошло не так') + ': ' + err
}

/**
 * @class
 * @typedef SN_State
 * @prop {boolean} isLoading
 * @prop {string} logText
 * @prop {number|null} count
 * @prop {import('src/api').NodeIPSanction|null} sanction
 * @extends {PureComponent<{}, SN_State>}
 */
export class SearchNeighbors extends PureComponent {
	constructor() {
		super()
		/** @type {SN_State} */
		this.state = { isLoading: false, logText: '', count: null, sanction: null }
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
		const address = (new FormData(e.target).get('address') + '').trim()
		if (address === '') return

		this.setState({ isLoading: true, logText: '', count: null, sanction: null })
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

		const neiPromise = apiReq('GET', `/api/neighbors/${subnet}.0`)
			.then(resp => {
				this.setState({ count: resp.count })
			})
			.catch(err => {
				if (err.error === 'WRONG_SUBNET_FORMAT') {
					this.addLogLines(L('wrong format', 'ru', 'неправильный формат'))
				} else {
					onError(err)
					this.addLogLines(unexpectedErrText(err))
				}
			})

		const sancIP = isIPv4(address) ? address : subnet ? `${subnet}.0` : null
		const sancPromise =
			!!sancIP &&
			apiReqIPsSanctions([sancIP], false, null)
				.then(resp => {
					this.setState({ sanction: resp.ips[sancIP]?.sanction ?? null })
				})
				.catch(err => {
					onError(err)
					this.addLogLines(unexpectedErrText(err))
				})

		await Promise.allSettled([neiPromise, sancPromise]).finally(() => {
			this.setState({ isLoading: false })
		})
	}

	render({}, /**@type {SN_State}*/ { isLoading, logText, count, sanction }) {
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
				${count === null
					? NBSP
					: lang === 'ru'
					? html`В подсети ${pluralize(count, 'нашлась', 'нашлось', 'нашлось')}${' '}
							<b>${L.n(count, 'нода', 'ноды', 'нод')}</b>${' '}
							<span class="dim">
								${pluralize(count, 'активная', 'активные', 'активных')} за последние 24 часа
							</span>`
					: html`<b>${L.n(count, 'node', 'nodes')}</b> ${pluralize(count, 'was', 'were')} found in
							the subnet${' '} <span class="dim"> reachable within the last 24 hours </span>`}
			</p>
			<p>
				${sanction
					? html`<span class="warn"><${NodeSanctionDescr} sanction=${sanction} /></span>,${' '}
							${L('possible payout problems', 'ru', 'возможны проблемы с выплатами')},${' '}
							<a href="/sanctions">${L('more info', 'ru', 'подробнее')}</a>`
					: null}
			</p>
			<p>${isLoading ? L('Loading…', 'ru', 'Загрузка…') : null}</p>
			${logText && html`<pre>${logText}</pre>`}
			<br />
			<${SubnetNeighborsDescription} classes="dim" />
		`
	}
}
