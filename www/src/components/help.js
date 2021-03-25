import { bindHandlers } from '../utils/elems'
import { html } from '../utils/htm'
import { createPortal, PureComponent } from '../utils/preact_compat'

import './help.css'

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

export class Help extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.state = { isShown: false }
	}

	onClick() {
		this.setState({ isShown: true })
	}
	onPopupClose() {
		this.setState({ isShown: false })
	}

	render({ contentFunc }, { isShown }) {
		return html`
			<button class="help" onclick=${this.onClick}>?</button>
			${isShown &&
			createPortal(
				html`<${Popup} onClose=${this.onPopupClose}>${contentFunc()}</${Popup}>`,
				document.body,
			)}
		`
	}
}
