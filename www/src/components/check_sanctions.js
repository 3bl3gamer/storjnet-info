import { useCallback, useState } from 'preact/hooks'
import { apiReqIPsSanctions } from 'src/api'
import { L, lang } from 'src/i18n'
import { html } from 'src/utils/htm'
import { NodeSanctionGeneralDescrPP, NodeSanctionDescr } from 'src/utils/nodes'

import './check_sanctions.css'
import { resolveAllMixed } from 'src/utils/dns'

export function CheckSanctions() {
	const [isLoading, setIsLoading] = useState(false)
	const [requestAddrs, setRequestAddrs] = useState(/**@type {string[]}*/ ([]))
	const [resolvedIPs, setResolvedIPs] = useState(/**@type {(string|Error)[]|null}*/ (null))
	const [response, setResponse] = useState(
		/**@type {import('src/api').IPsSanctionsResponse|Error|null}*/ (null),
	)

	const onSubmit = useCallback(e => {
		e.preventDefault()

		const addrsStr = (new FormData(e.target).get('ips') + '').trim()
		if (addrsStr === '') return
		const addrs = addrsStr.split(/[\s,]+/)

		setRequestAddrs(addrs)
		setResolvedIPs(null)
		setResponse(null)
		setIsLoading(true)

		resolveAllMixed(addrs)
			.then(ipOrErrs => {
				setResolvedIPs(ipOrErrs)
				const ips = ipOrErrs.filter(x => typeof x === 'string')
				return apiReqIPsSanctions(ips, true, null)
			})
			.then(res => {
				setResponse(res)
			})
			.catch(err => {
				setResponse(err)
			})
			.finally(() => {
				setIsLoading(false)
			})
	}, [])

	return html`<div class="p-like">
			<form class="check-sanctions-form" onsubmit=${onSubmit}>
				<textarea
					name="ips"
					placeholder=${lang === 'ru'
						? 'IP/домены\nчерез пробел/перенос/запятую'
						: 'IP/domains\nspace/newline/comma-separated'}
					cols="32"
					rows="4"
				/>${' '}
				<input type="submit" value="OK" disabled=${isLoading} />
			</form>
		</div>
		${response instanceof Error
			? html`<p class="warn">${response.message}</p>`
			: html`<${IPsTable} addrs=${requestAddrs} resolvedIPs=${resolvedIPs} resp=${response} />`}
		<${NodeSanctionGeneralDescrPP} />`
}

/** @param {{addrs: string[], resolvedIPs:(string|Error)[]|null, resp: import('src/api').IPsSanctionsResponse|null}} props */
function IPsTable({ addrs, resolvedIPs, resp }) {
	if (addrs.length === 0) return null

	let maxSubdivsCount = 0
	if (resp)
		for (const ip in resp.ips) {
			const len = resp.ips[ip].fullInfo?.subdivisions.length
			if (len && len > maxSubdivsCount) maxSubdivsCount = len
		}

	return html`<table class="check-sanctions-result-table underlined wide-padded">
		<tr>
			<th>IP</th>
			<th>${L('Reason', 'ru', 'Причина')}</th>
			<th>${L('Registration', 'ru', 'Регистрация')}</th>
			<th>${L('Country', 'ru', 'Страна')}</th>
			<th>${L('Region', 'ru', 'Регион')}</th>
			<th>${L('City', 'ru', 'Город')}</th>
		</tr>
		${addrs.map((addr, addrI) => {
			const ip = resolvedIPs?.[addrI]

			const info = typeof ip === 'string' ? resp?.ips[ip] : null
			const sanc = info?.sanction
			const full = info?.fullInfo

			return html`<tr>
				<td class="ip-addr">
					<div>${addr}</div>
					${ip === addr
						? null
						: ip instanceof Error
						? html`<div class="small warn">${ip.message}</div>`
						: html`<div class="small dim">${ip ?? ''}</div>`}
				</td>
				<td class="sanction">
					<div class="${sanc ? 'warn-no-pad' : ''}">
						${sanc ? html`<${NodeSanctionDescr} sanction=${sanc} />` : null}
					</div>
				</td>
				<td>
					<${GeoLabel} item=${full?.registeredCountry} />
				</td>
				<td><${GeoLabel} item=${full?.country} /></td>
				${Array(maxSubdivsCount)
					.fill(0)
					.map(i => html`<td><${GeoLabel} item=${full?.subdivisions[i]} /></td>`)}
				<td><${GeoLabel} item=${full?.city} /></td>
			</tr>`
		})}
	</table>`
}

/** @param {{item: {name:string, geoNameID:number, isoCode?:string}|undefined}} props */
function GeoLabel({ item }) {
	if (!item || (item.name === '' && item.geoNameID === 0)) return '—'
	return html`<div>${item.name}</div>
		<span class="small dim">${item.isoCode ?? ''}#${item.geoNameID}</span>`
}
