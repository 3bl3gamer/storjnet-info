import { h } from 'preact'
import { PureComponent, renderIfExists, html, bindHandlers, onError } from './utils'

import './auth.css'
import { apiReq } from './api'

const lang = 'ru'

class AuthForm extends PureComponent {
	constructor() {
		super()
		bindHandlers(this)
		this.state = { mode: 'register', emailError: null }
	}

	register(form) {
		this.setState({ emailError: null })
		apiReq('POST', '/api/register', { data: Object.fromEntries(new FormData(form)) })
			.then(res => {
				location.href = '/~'
			})
			.catch(err => {
				if (err.error == 'WRONG_EMAIL')
					this.setState({
						emailError: lang == 'ru' ? 'неправильнй адрес' : 'wrong address',
					})
				else if (err.error == 'EMAIL_EXISTS')
					this.setState({
						emailError: lang == 'ru' ? 'адрес занят' : 'address not available',
					})
				else onError(err)
			})
	}
	login(form) {
		this.setState({ emailError: null })
		apiReq('POST', '/api/login', { data: Object.fromEntries(new FormData(form)) })
			.then(res => {
				location.href = '/~'
			})
			.catch(err => {
				if (err.error == 'WRONG_EMAIL_OR_PASSWORD')
					this.setState({
						emailError:
							lang == 'ru' ? 'неправильная почта или пароль' : 'wrong e-mail or password',
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
			if (data.get('email') != '' && data.get('password') != '') {
				if (form.checkValidity()) this[e.target.name](form)
			}
		}
	}

	render(props, { mode, emailError }) {
		const regButType = mode == 'register' ? 'submit' : 'button'
		const logButType = mode == 'login' ? 'submit' : 'button'
		return html`
			<form class="registration-form" onsubmit=${this.onSubmit}>
				<div class="email-error">${emailError}</div>
				<input type="email" name="email" required placeholder="E-mail" />
				<input type="password" name="password" required placeholder="Password" />
				<div class="buttons-wrap">
					<button type=${regButType} name="register" onclick=${this.onClick}>
						Регистрация
					</button>
					<button type=${logButType} name="login" onclick=${this.onClick}>
						Вход
					</button>
				</div>
			</form>
		`
	}
}

renderIfExists(h(AuthForm), '.auth-forms')
