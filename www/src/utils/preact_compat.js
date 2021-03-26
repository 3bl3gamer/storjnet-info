// direct imports to avoid kilobytes of side-effects from 'preact/compat'
import { PureComponent as PureComponent_ } from 'preact/compat/src/PureComponent'
import { createPortal as createPortal_ } from 'preact/compat/src/portals'

// overriding type definition by preact/compat/index.d.ts since src/PureComponent is typed incorrectly
export const PureComponent = /** @type {typeof import('preact/compat').PureComponent} */ (
	/** @type {*} */ (PureComponent_)
)

export const createPortal = /** @type {typeof import('preact/compat').createPortal} */ (createPortal_)
