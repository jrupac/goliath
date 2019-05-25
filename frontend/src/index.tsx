import React from "react";
import ReactDOM from "react-dom";
import {BrowserRouter, Route, Switch} from 'react-router-dom';
import App from './App';
import Login from './components/Login';
import * as registerServiceWorker from "./registerServiceWorker";

registerServiceWorker.register();

ReactDOM.render(
  <BrowserRouter>
    <div className="App">
      <Switch>
        <Route exact path='/login' component={Login}/>
        <Route path='/' component={App}/>
      </Switch>
    </div>
  </BrowserRouter>,
  document.getElementById('root')
);
