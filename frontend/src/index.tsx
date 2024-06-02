import React from "react";
import {createRoot} from 'react-dom/client';
import {BrowserRouter, Route, Switch} from 'react-router-dom';
import App from './App';
import Login from './components/Login';

const container = document.getElementById('root');
const root = createRoot(container!);

root.render(
  <BrowserRouter>
    <div className="App">
      <Switch>
        <Route exact path='/login' component={Login}/>
        <Route path='/' component={App}/>
      </Switch>
    </div>
  </BrowserRouter>);