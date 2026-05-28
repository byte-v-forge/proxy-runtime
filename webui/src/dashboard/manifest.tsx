import { Network } from 'lucide-react';
import { DashboardNavSection, type DashboardModuleRegistration } from '@byte-v-forge/common-ui';
import { ProxyRuntimePage } from './proxy-runtime-page';
import './styles.css';

const registration: DashboardModuleRegistration = {
  manifest: {
    id: 'proxy-runtime',
    nav: [
      {
        key: 'proxy-runtime',
        label: '出口网关',
        icon: 'proxy-runtime',
        section: DashboardNavSection.DASHBOARD_NAV_SECTION_INFRASTRUCTURE,
        required_services: ['proxy-runtime'],
        order: 28
      }
    ]
  },
  icons: {
    'proxy-runtime': <Network size={17} />
  },
  views: {
    'proxy-runtime': () => <ProxyRuntimePage />
  }
};

export default registration;
