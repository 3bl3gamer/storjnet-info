import { h } from 'preact'
import { bindHandlers } from 'src/utils/elems'
import { html } from 'src/utils/htm'
import { createPortal, PureComponent } from 'src/utils/preact_compat'

import './help.css'

/**
 * @class
 * @typedef P_Props
 * @prop {() => void} onClose
 * @extends {PureComponent<P_Props, {}>}
 */
class Popup extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
	}

	componentDidMount() {
		addEventListener('keydown', this.onKeyDown)
	}
	componentWillUnmount() {
		removeEventListener('keydown', this.onKeyDown)
	}

	onKeyDown(e) {
		if (e.key === 'Escape') {
			e.preventDefault()
			this.props.onClose()
		}
	}
	onBackgroundClick(e) {
		if (e.target.classList.contains('popup')) {
			this.props.onClose()
		}
	}

	render({ children, onClose }) {
		return html`
			<div class="popup" onclick=${this.onBackgroundClick}>
				<div class="popup-frame">
					<button class="popup-close" onclick=${onClose}>✕</button>
					<div class="popup-content">${children}</div>
				</div>
				<div></div>
			</div>
		`
	}
}

/**
 * @class
 * @typedef H_Props
 * @prop {() => import('preact').JSX.Element} contentFunc
 * @typedef H_State
 * @prop {boolean} isShown
 * @extends {PureComponent<H_Props, {}>}
 */
export class Help extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		/** @type {H_State} */
		this.state = { isShown: false }
	}

	onClick() {
		this.setState({ isShown: true })
	}
	onPopupClose() {
		this.setState({ isShown: false })
	}

	/**
	 * @param {H_Props} props
	 * @param {H_State} state
	 */
	render({ contentFunc }, { isShown }) {
		return html`
			<button class="help" onclick=${this.onClick}>?</button>
			${isShown &&
			createPortal(
				h(Popup, { onClose: this.onPopupClose }, contentFunc()), //
				document.body,
			)}
		`
	}
}
