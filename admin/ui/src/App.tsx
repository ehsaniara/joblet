import {BrowserRouter as Router, Route, Routes} from 'react-router-dom';
import Layout from './components/Layout/Layout';
import Dashboard from './pages/Dashboard';
import Jobs from './pages/Jobs';
import CreateJob from './pages/CreateJob';
import Workflows from './pages/Workflows';
import Monitoring from './pages/Monitoring';
import Resources from './pages/Resources';

function App() {
    return (
        <Router>
            <Routes>
                <Route path="/jobs/create" element={<CreateJob/>}/>
                <Route path="*" element={
                    <Layout>
                        <Routes>
                            <Route path="/" element={<Dashboard/>}/>
                            <Route path="/jobs" element={<Jobs/>}/>
                            <Route path="/workflows" element={<Workflows/>}/>
                            <Route path="/monitoring" element={<Monitoring/>}/>
                            <Route path="/resources" element={<Resources/>}/>
                        </Routes>
                    </Layout>
                }/>
            </Routes>
        </Router>
    );
}

export default App;