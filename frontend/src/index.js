import React from 'react';
import ReactDOM from 'react-dom';
import { Router, Route, browserHistory } from 'react-router';
import App from './App';
import Login from './components/Login';

ReactDOM.render((
    <Router history={browserHistory}>
      <div className="App">
        <Route path='/login' component={Login} />
        <Route path='/' component={App} />
      </div>
    </Router>),
    document.getElementById('root')
);
