import React from 'react';
import ReactDOM from 'react-dom';
import {BrowserRouter, Route} from 'react-router-dom';
import App from './App';
import Login from './components/Login';

ReactDOM.render(
  <BrowserRouter>
    <div className="App">
      <Route path='/login' component={Login}/>
      <Route path='/' component={App}/>
    </div>
  </BrowserRouter>,
  document.getElementById('root')
);
