import { memo } from 'src/utils/preact_compat'
import { html } from 'src/utils/htm'
import { L, lang } from 'src/i18n'
import { useCallback, useEffect, useMemo, useState } from 'preact/hooks'
import { apiReq } from 'src/api'
import { toISODateString, useHashInterval } from 'src/utils/time'
import { onError } from 'src/errors'
import { zeroes } from 'src/utils/arrays'

import './nodes_subnet_summary.css'

const SIZES_STATS_COUNTS = [1, 2, 3, 10, 100]

const NodesSummary = memo(function NodesSummary() {
	const [stats, setStats] = useState(
		/**
		 * @type {{
		 *   subnetsCount: number,
		 *   subnetsTop: {subnet:string, size:number}[],
		 *   subnetSizes: {size:number, count:number}[],
		 * } | null}
		 */ (null),
	)
	const [isExpanded, setIsExpanded] = useState(false)
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

	const onExpand = useCallback(() => {
		setIsExpanded(true)
	}, [])

	const sizesStats = useMemo(() => {
		if (stats === null) return null

		if (isExpanded) {
			return stats.subnetSizes.map(x => ({ label: x.size + '', count: x.count }))
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
					count: stats.subnetSizes
						.filter(x => x.count >= count && x.count < nextCount)
						.map(x => x.size)
						.reduce((a, b) => a + b, 0),
				}
			})
		}
	}, [stats, isExpanded])

	const subnetsCount = stats ? stats.subnetsCount : 0

	return html`
		<div class="node-subnets-summary-wrap p-like">
			<table class="node-subnets-table underlined wide-padded">
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
					${stats === null
						? zeroes(5).map(
								(_, i) => html`<tr>
									<td class="dim">${i + 1}</td>
									<td class="dim">${L('loading...', 'ru', 'загрузка...')}</td>
									<td class="dim">...</td>
								</tr>`,
						  )
						: stats.subnetsTop.map(
								(item, i) =>
									html`<tr>
										<td>${i + 1}</td>
										<td><${Subnet} subnet=${item.subnet} /></td>
										<td>${item.size}</td>
									</tr>`,
						  )}
					<tr>
						<td></td>
						<td>...</td>
						<td></td>
					</tr>
				</tbody>
			</table>
			<table class="node-subnet-sizes-table underlined wide-padded">
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
						? zeroes(SIZES_STATS_COUNTS).map(
								x =>
									html`<tr>
										${x}
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
						html`
							<td colspan="3">
								<button class="unwrap-button" onclick=${onExpand}>
									${L('Expand', 'ru', 'Развернуть')}
								</button>
							</td>
						`}
					</tr>
				</tbody>
			</table>
		</div>
		<p>
			${lang === 'ru'
				? `Ноды запущены как минимум в ${L.n(subnetsCount, 'подсети', 'подсетях', 'подсетях')}.`
				: `Nodes are running in at least ${L.n(subnetsCount, 'subnet', 'subnets')}.`}
		</p>
	`
})

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
