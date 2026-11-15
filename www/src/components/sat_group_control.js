import { L } from 'src/i18n'
import { html } from 'src/utils/htm'
import { getSatGroups, makeUpdatedHashSatGroup, satGroupLabel, useHashSatGroup } from 'src/utils/sat_groups'

import './sat_group_control.css'

/** Chooses the satellites group whose stats are shown on the page. */
export function SatGroupControl() {
	const curGroup = useHashSatGroup()
	const groups = getSatGroups()

	// nothing to choose from
	if (groups.length < 2) return null

	return html`<p class="sat-group-control">
		${L('Satellites', 'ru', 'Спутники')}:${' '}
		${groups.map(
			group =>
				html`<button
					class="${group === curGroup ? 'active' : ''}"
					aria-pressed="${group === curGroup}"
					onClick=${() => {
						// the hash also holds the interval, so making it at the moment of the click
						location.hash = makeUpdatedHashSatGroup(group)
					}}
				>
					${satGroupLabel(group)}
				</button>`,
		)}
	</p>`
}
