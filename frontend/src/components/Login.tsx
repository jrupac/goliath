import React, {FormEvent, ReactNode} from 'react';
import Form from 'antd/lib/form';
import Input from 'antd/lib/input';
import Button from 'antd/lib/button';
import {RouteComponentProps, withRouter} from "react-router-dom";

// WrappedLoginProps needs to extend RouteComponentProps to get "history".
export interface WrappedLoginProps extends RouteComponentProps {
}

class WrappedLogin extends React.Component<WrappedLoginProps, any> {
  constructor(props: WrappedLoginProps) {
    super(props);
    this.state = {loginFailed: false};
  }

  render() {
    return (
      <div className='login-page'>
        <div className='login-page-logo'>Goliath</div>
        <div className='login-page-form-background'>
          <Form
            id="login-form"
            onFinish={(e: FormEvent) => this.handleSubmit(e)}
            layout='vertical'
            className='login-page-form'>
            {this.showLoginFailedMessage()}

            <Form.Item
              name="username"
              rules={[{required: true, message: 'Empty username!'}]}>
              <Input
                prefix={<i
                  className="fas fa-user"
                  aria-hidden="true"/>}
                placeholder="username"/>
            </Form.Item>
            <Form.Item
              name="password"
              rules={[{required: true, message: 'Empty password!'}]}>
              <Input
                prefix={<i
                  className="fas fa-lock"
                  aria-hidden="true"/>}
                type="password"
                placeholder="password"/>
            </Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              className="login-page-form-button">
              Log in
            </Button>
          </Form>
        </div>
      </div>
    )
  }

  showLoginFailedMessage = (): ReactNode => {
    if (this.state.loginFailed) {
      return <div className="login-failed">Invalid username or password!</div>
    } else {
      return null;
    }
  };

  handleSubmit(e: React.FormEvent) {
    fetch('/auth', {
      method: 'POST',
      body: JSON.stringify(e),
      credentials: 'include'
    }).then((res: Response) => {
      if (!res.ok) {
        this.setState({loginFailed: true});
        console.log(res);
      } else {
        this.setState({loginFailed: false});
        this.props.history.push({
          pathname: '/'
        });
      }
    }).catch((e) => {
      this.setState({loginFailed: true});
      console.log(e);
    });
  }
}

export default withRouter(WrappedLogin);
