import { lang } from '../i18n'
import { html } from '../utils/htm'
import { PureComponent } from '../utils/preact_compat'
import { endOfMonth, makeUpdatedHashInterval, watchHashInterval } from '../utils/time'

const monthLangNames = {
	en: 'january february march april may june july august september october november december'.split(' '),
	ru: 'январь февраль март апрель май июнь июль август сентябрь октябрь ноябрь декабрь'.split(' '),
}

export class RewindControl extends PureComponent {
	constructor() {
		super()

		let watch = watchHashInterval((startDate, endDate) => {
			this.setState({ ...this.state, startDate, endDate })
		})
		this.stopWatchingHashInterval = watch.off

		this.state = {
			startDate: watch.startDate,
			endDate: watch.endDate,
		}
	}

	makeIntervalHash(monthDelta) {
		let startDate = new Date(this.state.startDate)
		startDate.setUTCMonth(startDate.getUTCMonth() + monthDelta)

		let endDate = new Date(this.state.endDate)
		endDate.setUTCMonth(endDate.getUTCMonth() + monthDelta)

		const hasSwitchedExtraMonth =
			Math.abs(monthDelta + 12) % 12 !==
			(this.state.endDate.getUTCMonth() - endDate.getUTCMonth() + 24) % 12
		if (hasSwitchedExtraMonth) {
			// so that 01-31 + 1 month = 02-28 but not 03-03
			endDate.setDate(endDate.getUTCDate() - 10)
			endDate = endOfMonth(endDate)
		}
		return makeUpdatedHashInterval(startDate, endDate)
	}

	render(props, { startDate, endDate }) {
		let monthNames = monthLangNames[lang] || monthLangNames.en

		let curMonthName = monthNames[startDate.getUTCMonth()]
		if (startDate.getUTCMonth() !== endDate.getUTCMonth())
			curMonthName += ' — ' + monthNames[endDate.getUTCMonth()]

		let prevMonthName = monthNames[(startDate.getUTCMonth() + 11) % 12]
		let nextMonthName = monthNames[(endDate.getUTCMonth() + 1) % 12]

		return html`<p>
			<a href="${this.makeIntervalHash(-1)}">← ${prevMonthName}</a>
			${' '}<b>${curMonthName}</b>${' '}
			<a href="${this.makeIntervalHash(1)}">${nextMonthName} →</a>
		</p>`
	}
}
