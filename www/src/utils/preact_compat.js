// direct imports to avoid kilobytes of side-effects from 'preact/compat'
import { PureComponent as PureComponent_ } from 'preact/compat/src/PureComponent'
import { createPortal as createPortal_ } from 'preact/compat/src/portals'
import { memo as memo_ } from 'preact/compat/src/memo'

// overriding type definition by preact/compat/index.d.ts since src/PureComponent is typed incorrectly
export const PureComponent = /** @type {typeof import('preact/compat').PureComponent} */ (
	/** @type {*} */ (PureComponent_)
)

// completelly overriding type: createPortal actually CAN accept array in first argument
export const createPortal =
	/** @type {(vnode: preact.VNode<{}>|preact.VNode<{}>[], container: Element) => preact.VNode<any>} */ (
		createPortal_
	)

/**
 * @template TProps
 * @template {import('preact').VNode | import('preact').VNode[] | null} TRet
 * @template {(props:TProps) => TRet} C
 * @param {C} c
 * @param {(prev:TProps, next:TProps) => boolean} [comparer]
 * @returns {C}
 */
export function memo(c, comparer) {
	// extra wrapper, because JSDoc @function does not work with plain memo_
	return /** @type {*} */ (memo_)(c, comparer)
}
