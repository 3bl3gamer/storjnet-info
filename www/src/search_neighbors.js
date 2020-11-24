import { h } from 'preact'
import { apiReq } from './api'
import { L, lang, pluralize } from './i18n'
import { bindHandlers, html, onError, PureComponent, renderIfExists } from './utils'

function wrongFormat() {
	return L('wrong format', 'ru', 'неправильный формат')
}

class SearchNeighbors extends PureComponent {
	constructor() {
		super()
		this.state = { isLoading: false, error: null, count: null }
		this.abortController = null
		bindHandlers(this)
	}

	onSubmit(e) {
		e.preventDefault()
		const value = new FormData(e.target).get('subnet') || ''
		const m = value.trim().match(/^\d+\.\d+\.\d+/)
		if (m === null) {
			this.setState({ error: wrongFormat() })
			return
		}
		const subnet = m[0]

		this.setState({ isLoading: true, error: null, count: null })
		apiReq('GET', `/api/neighbors/${subnet}.0`)
			.then(resp => {
				this.setState({ count: resp.count })
			})
			.catch(err => {
				if (err.error === 'WRONG_SUBNET_FORMAT') {
					this.setState({ error: wrongFormat() })
				} else {
					onError(err)
					this.setState({ error: err + '' })
				}
			})
			.finally(() => {
				this.setState({ isLoading: false })
			})
	}

	render({}, { isLoading, error, count }) {
		return html`
			<p>
				<form class="search-neighbors-form" onSubmit=${this.onSubmit}>
					<input
						class="subnet-input"
						placeholder=${L('IP/subnet: 1.2.3.4 or 1.2.3', 'ru', 'IP/подсеть: 1.2.3.4 или 1.2.3')}
						name="subnet"
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
						: error
						? error
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
