import { useCallback, useLayoutEffect, useState } from 'preact/hooks'
import { html } from 'src/utils/htm'
import { createPortal } from 'src/utils/preact_compat'

import './help.css'

/**
 * @param {{
 *   onClose(): void,
 *   children: import('preact').JSX.Element
 * }} props
 */
export function Popup({ onClose, children }) {
	const onKeyDown = useCallback(
		e => {
			if (e.key === 'Escape') {
				e.preventDefault()
				onClose()
			}
		},
		[onClose],
	)
	const onBackgroundClick = useCallback(
		e => {
			if (e.target.classList.contains('popup')) {
				onClose()
			}
		},
		[onClose],
	)

	useLayoutEffect(() => {
		addEventListener('keydown', onKeyDown)
		return () => removeEventListener('keydown', onKeyDown)
	}, [onKeyDown])

	return html`
		<div class="popup" onclick=${onBackgroundClick}>
			<div class="popup-frame">
				<button class="popup-close" onclick=${onClose}>✕</button>
				<div class="popup-content">${children}</div>
			</div>
			<div>
				<!-- this div moves popup-frame a bit higher -->
			</div>
		</div>
	`
}

/**
 * @param {{
 *   contentFunc(): import('preact').JSX.Element,
 *   letter?: string
 * }} props
 */
export function Help({ contentFunc, letter = '?' }) {
	const [isShown, setIsShown] = useState(false)

	const onClick = useCallback(() => {
		setIsShown(true)
	}, [setIsShown])
	const onPopupClose = useCallback(() => {
		setIsShown(false)
	}, [setIsShown])

	return html`
		<button class="help" onclick=${onClick}>${letter}</button>
		${isShown &&
		createPortal(
			html`<${Popup} onClose=${onPopupClose}>${contentFunc()}</${Popup}>`, //
			document.body,
		)}
	`
}

/**
 * @param {{
 *   classes?: string
 *   contentFunc(): import('preact').JSX.Element,
 *   children: import('preact').JSX.Element
 * }} props
 */
export function HelpLine({ classes, contentFunc, children }) {
	const [isShown, setIsShown] = useState(false)

	const onClick = useCallback(() => {
		setIsShown(true)
	}, [setIsShown])
	const onPopupClose = useCallback(() => {
		setIsShown(false)
	}, [setIsShown])

	return html`
		<button class="help-line ${classes ?? ''}" onclick=${onClick}>${children}</button>
		${isShown &&
		createPortal(
			html`<${Popup} onClose=${onPopupClose}>${contentFunc()}</${Popup}>`, //
			document.body,
		)}
	`
}
