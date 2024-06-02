import React from "react";
import {createRoot} from 'react-dom/client';
import {BrowserRouter, Route, Routes} from 'react-router-dom';
import App from './App';
import Login from './components/Login';

const container = document.getElementById('root');
const root = createRoot(container!);

root.render(
  <BrowserRouter>
    <div className="App">
      <Routes>
        <Route path='/login' element={<Login/>}/>
        <Route path='/' element={<App/>}/>
      </Routes>
    </div>
  </BrowserRouter>);