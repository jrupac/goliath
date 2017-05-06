import React from 'react';
import ReactDOM from 'react-dom';
import { BrowserRouter as Router, Route } from 'react-router-dom';
import App from './App';
import Login from './components/Login';

ReactDOM.render((
    <Router>
      <div className="App">
        <Route exact path='/login' component={Login} />
        <Route exact path='/' component={App} />
      </div>
    </Router>),
    document.getElementById('root')
);
