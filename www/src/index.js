import './errors'
import './shame'

import './main.css'

import { renderIfExists } from './utils/elems'
import { StorjTxSummary } from './components/storj_tx_summary'
import { NodesLocationSummary } from './components/nodes_location_summary'
import { NodesCountChart } from './components/nodes_count_chart'
import { RewindControl } from './components/rewind_control'
import { SatsPingsCharts } from './components/pings_chart'
import { PingMyNode } from './components/ping_my_node'
import { AuthForm } from './components/auth'
import { SearchNeighbors } from './components/search_neighbors'
import { CheckSanctions } from './components/check_sanctions'
import { UserDashboardNodes, UserDashboardPings } from './components/user_dashboard'
import { NodesSubnetSummary } from './components/nodes_subnet_summary'

renderIfExists(AuthForm, '.auth-forms')
renderIfExists(RewindControl, '.rewind-control')
renderIfExists(SatsPingsCharts, '.sat-nodes')
renderIfExists(StorjTxSummary, '.storj-tx-summary')
renderIfExists(NodesCountChart, '.nodes-count-chart')
renderIfExists(NodesLocationSummary, '.nodes-location-summary')
renderIfExists(NodesSubnetSummary, '.nodes-subnet-summary')
renderIfExists(PingMyNode, '.ping-my-node')
renderIfExists(SearchNeighbors, '.search-neighbors')
renderIfExists(CheckSanctions, '.check-sanctions')
renderIfExists(UserDashboardNodes, '.user-dashboard-nodes')
renderIfExists(UserDashboardPings, '.user-dashboard-pings')
