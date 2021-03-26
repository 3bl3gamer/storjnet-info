import { PureComponent } from 'src/utils/preact_compat'
import { onError } from 'src/errors'
import { bindHandlers } from 'src/utils/elems'
import { apiReq } from 'src/api'
import { html } from 'src/utils/htm'
import { L } from 'src/i18n'

import './auth.css'

/**
 * @class
 * @typedef AF_State
 * @prop {'register'|'login'} mode
 * @prop {string|null} authError
 * @extends {PureComponent<{}, AF_State>}
 */
export class AuthForm extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		/** @type {AF_State} */
		this.state = { mode: 'login', authError: null }
	}

	register(form) {
		this.setState({ authError: null })
		apiReq('POST', '/api/register', { data: Object.fromEntries(new FormData(form)) })
			.then(res => {
				location.href = '/~'
			})
			.catch(err => {
				if (err.error == 'USERNAME_TO_SHORT')
					this.setState({
						authError: L('username to short', 'ru', 'логин слишком короткий'),
					})
				else if (err.error == 'USERNAME_EXISTS')
					this.setState({
						authError: L('username not available', 'ru', 'логин занят'),
					})
				else onError(err)
			})
	}
	login(form) {
		this.setState({ authError: null })
		apiReq('POST', '/api/login', { data: Object.fromEntries(new FormData(form)) })
			.then(res => {
				location.href = '/~'
			})
			.catch(err => {
				if (err.error == 'WRONG_USERNAME_OR_PASSWORD')
					this.setState({
						authError: L('wrong username or password', 'ru', 'неправильный логин или пароль'),
					})
				else onError(err)
			})
	}

	onSubmit(e) {
		e.preventDefault()
		this[this.state.mode](e.target)
	}
	onClick(e) {
		if (e.target.name != this.state.mode) {
			e.preventDefault() //иначе на инпутах срабатывает валидация
			let form = e.target.closest('form')
			let data = new FormData(form)
			this.setState({ mode: e.target.name })
			if (data.get('username') != '' && data.get('password') != '') {
				if (form.checkValidity()) this[e.target.name](form)
			}
		}
	}

	/**
	 * @param {{}} props
	 * @param {AF_State} state
	 */
	render(props, { mode, authError }) {
		const regButType = mode == 'register' ? 'submit' : 'button'
		const logButType = mode == 'login' ? 'submit' : 'button'
		return html`
			<form class="registration-form" onsubmit=${this.onSubmit}>
				<div class="auth-error">${authError}</div>
				<input type="text" name="username" required placeholder="${L('Username', 'ru', 'Логин')}" />
				<input
					type="password"
					name="password"
					required
					placeholder="${L('Password', 'ru', 'Пароль')}"
				/>
				<div class="buttons-wrap">
					<button type=${regButType} name="register" onclick=${this.onClick}>
						${L('Register', 'ru', 'Регистрация')}
					</button>
					<button type=${logButType} name="login" onclick=${this.onClick}>
						${L('Login', 'ru', 'Вход')}
					</button>
				</div>
			</form>
		`
	}
}
