import { memo } from 'src/utils/preact_compat'
import { html } from 'src/utils/htm'
import { L, lang } from 'src/i18n'
import { useCallback, useEffect, useMemo, useState } from 'preact/hooks'
import { apiReq } from 'src/api'
import { toISODateString, useHashInterval } from 'src/utils/time'
import { onError } from 'src/errors'
import { zeroes } from 'src/utils/arrays'

import './nodes_subnet_summary.css'
import { THINSP } from 'src/utils/elems'

const SIZES_STATS_COUNTS = [1, 2, 3, 10, 100]

/**
 * @typedef {{
 *   subnetsCount: number,
 *   subnetsTop: {subnet:string, size:number}[],
 *   subnetSizes: {size:number, count:number}[],
 *   ipTypes: {type:string, count:number, asnTop:{name:string, count:number}[]}[],
 * }} NodesSubnetsSummaryResponse
 */

const NodesSummary = memo(function NodesSummary() {
	const [stats, setStats] = useState(/** @type {NodesSubnetsSummaryResponse | null} */ (null))
	const [, endDate] = useHashInterval()

	useEffect(() => {
		const abortController = new AbortController()

		apiReq('GET', `/api/nodes/subnet_summary`, {
			data: { end_date: toISODateString(endDate) },
			signal: abortController.signal,
		})
			.then(stats => {
				setStats(stats)
			})
			.catch(onError)

		return () => abortController.abort()
	}, [endDate])

	const subnetsCount = stats ? stats.subnetsCount : 0

	return html`
		<div class="node-subnets-summary-wrap p-like">
			<${NodesSubnetsSummaryTable} subnetsTop=${stats?.subnetsTop} />
			<${NodesSubnetsSizesTable} subnetSizes=${stats?.subnetSizes} />
			<${NodesIPTypesTable} ipTypes=${stats?.ipTypes} />
		</div>
		<p>
			${lang === 'ru'
				? `Ноды запущены как минимум в ${L.n(subnetsCount, 'подсети', 'подсетях', 'подсетях')} /24.`
				: `Nodes are running in at least ${L.n(subnetsCount, 'subnet', 'subnets')} /24.`}
		</p>
	`
})

/** @param {{subnetsTop: undefined | NodesSubnetsSummaryResponse['subnetsTop']}} props */
function NodesSubnetsSummaryTable({ subnetsTop }) {
	return html`<table class="node-subnets-table underlined wide-padded">
		<thead>
			<tr>
				<td>${L('#', 'ru', '№')}</td>
				<td>${L('Subnet', 'ru', 'Подсеть')}</td>
				<td>
					${L('Nodes', 'ru', 'Нод')}
					<div class="small dim">${L('in subnet', 'ru', 'в подсети')}</div>
				</td>
			</tr>
		</thead>
		<tbody>
			${!subnetsTop
				? zeroes(5).map(
						(_, i) => html`<tr>
							<td class="dim">${i + 1}</td>
							<td class="dim">${L('loading…', 'ru', 'загрузка…')}</td>
							<td class="dim">…</td>
						</tr>`,
				  )
				: subnetsTop.map(
						(item, i) =>
							html`<tr>
								<td>${i + 1}</td>
								<td><${Subnet} subnet=${item.subnet} /></td>
								<td>${item.size}</td>
							</tr>`,
				  )}
			<tr>
				<td></td>
				<td>…</td>
				<td></td>
			</tr>
		</tbody>
	</table>`
}

/** @param {{subnetSizes: undefined | NodesSubnetsSummaryResponse['subnetSizes']}} props */
function NodesSubnetsSizesTable({ subnetSizes }) {
	const [isExpanded, setIsExpanded] = useState(false)

	const onExpand = useCallback(() => {
		setIsExpanded(true)
	}, [])

	const sizesStats = useMemo(() => {
		if (!subnetSizes) return null

		if (isExpanded) {
			return subnetSizes.map(x => ({ label: x.size + '', count: x.count }))
		} else {
			return SIZES_STATS_COUNTS.map((count, i, counts) => {
				const nextCount = counts[i + 1] || Infinity
				return {
					label:
						count === nextCount - 1
							? count + ''
							: nextCount === Infinity
							? count + '+'
							: html`${count}<span class="dim">–${nextCount - 1}</span>`,
					count: subnetSizes
						.filter(x => x.size >= count && x.size < nextCount)
						.map(x => x.count)
						.reduce((a, b) => a + b, 0),
				}
			}).filter(x => x.count > 0)
		}
	}, [subnetSizes, isExpanded])

	return html`<table class="node-subnet-sizes-table underlined wide-padded">
		<thead>
			<tr>
				<td>
					${L('Nodes', 'ru', 'Нод')}
					<div class="small dim">${L('in subnet', 'ru', 'в подсети')}</div>
				</td>
				<td>
					${L('Count', 'ru', 'Кол-во')}
					<div class="small dim">${L('of subnets', 'ru', 'подсетей')}</div>
				</td>
			</tr>
		</thead>
		<tbody>
			${sizesStats === null
				? zeroes(SIZES_STATS_COUNTS.length).map(
						x =>
							html`<tr>
								<td class="dim" colspan="2">${L('loading…', 'ru', 'загрузка…')}</td>
							</tr>`,
				  )
				: sizesStats.map(
						(item, i) =>
							html`<tr>
								<td>${item.label}</td>
								<td>${item.count}</td>
							</tr>`,
				  )}
			<tr>
				${!isExpanded &&
				sizesStats &&
				sizesStats.length > 0 &&
				html`
					<td colspan="3">
						<button class="a-like" onclick=${onExpand}>${L('Expand', 'ru', 'Развернуть')}</button>
					</td>
				`}
			</tr>
		</tbody>
	</table>`
}

/** @param {{ipTypes: undefined | NodesSubnetsSummaryResponse['ipTypes']}} props */
function NodesIPTypesTable({ ipTypes }) {
	const [expanded, setExpanded] = useState(/**@type {Record<string, true>}*/ ({}))

	const toggle = useCallback((/**@type {string}*/ type) => {
		setExpanded(expanded => {
			expanded = { ...expanded }
			if (expanded[type]) {
				delete expanded[type]
			} else {
				expanded[type] = true
			}
			return expanded
		})
	}, [])

	return html`<table class="node-ip-types-table underlined wide-padded">
		<thead>
			<tr>
				<td>
					${Object.keys(expanded).length > 0 &&
					html`${L('Name', 'ru', 'Название')}
						<div class="small dim">${L('of AS', 'ru', "AS'ки")}</div>`}
				</td>
				<td>
					${L('Type', 'ru', 'Тип')}
					<div class="small dim">${L('of IP-addr', 'ru', 'IP-адреса')}</div>
				</td>
				<td>
					${L('Nodes', 'ru', 'Кол-во')}
					<div class="small dim">${L('count', 'ru', 'нод')}</div>
				</td>
			</tr>
		</thead>
		<tbody>
			${!ipTypes
				? zeroes(4).map(
						(_, i) => html`<tr>
							<td colspan="3" class="dim">${L('loading…', 'ru', 'загрузка…')}</td>
						</tr>`,
				  )
				: ipTypes.map(item => {
						const isExpanded = !!expanded[item.type]
						const isTruncated = item.count > item.asnTop.reduce((a, b) => a + b.count, 0)
						return html`<tr class="${isExpanded ? 'expanded' : ''}">
								<td></td>
								<td class="type">
									<button class="a-like" onclick=${() => toggle(item.type)}>
										${isExpanded ? '⏷' : '⏵'}${THINSP}${item.type}
									</button>
								</td>
								<td class="count">${item.count}</td>
							</tr>
							${isExpanded &&
							item.asnTop.map(
								x =>
									html`<tr>
										<td class="name" colspan="2">${x.name}</td>
										<td class="count">${x.count}</td>
									</tr>`,
							)}
							${isExpanded &&
							isTruncated &&
							html`<tr>
								<td></td>
								<td class="type dim">…</td>
								<td></td>
							</tr>`}`
				  })}
		</tbody>
	</table>`
}

/** @param {{subnet:string}} props */
function Subnet({ subnet }) {
	return subnet.endsWith('.0') ? html`${subnet.slice(0, -2)}<span class="dim">.0</span>` : subnet
}

export const NodesSubnetSummary = memo(function NodesSubnetSummary() {
	return html`
		<h3>${L('By subnets', 'ru', 'По подсетям')}</h3>
		<${NodesSummary} />
	`
})
