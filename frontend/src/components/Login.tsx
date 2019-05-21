import React, {FormEvent, ReactNode} from 'react';
import Form, {FormComponentProps} from 'antd/lib/form';
import Input from 'antd/lib/input';
import Button from 'antd/lib/button';
import {RouteComponentProps, withRouter} from "react-router-dom";

const FormItem = Form.Item;

// WrappedLoginProps needs to extend FormComponentProps to get the "form" prop
// and RouteComponentProps to get "history".
export interface WrappedLoginProps extends FormComponentProps, RouteComponentProps {
}

class WrappedLogin extends React.Component<WrappedLoginProps, any> {
  constructor(props: WrappedLoginProps) {
    super(props);
    this.state = {loginFailed: false};
  }

  render() {
    const {getFieldDecorator} = this.props.form;

    return (
      <div className='login-page'>
        <div className='login-page-logo'>Goliath</div>
        <div className='login-page-form-background'>
          <Form
            id="login-form"
            onSubmit={(e: FormEvent) => this.handleSubmit(e)}
            layout='vertical'
            className='login-page-form'>
            {this.showLoginFailedMessage()}
            <FormItem>
              {getFieldDecorator('username', {
                rules: [{required: true, message: 'Empty username!'}],
              })(<Input
                prefix={<i
                  className="fas fa-user"
                  aria-hidden="true"/>}
                placeholder="username"/>)}
            </FormItem>
            <FormItem>
              {getFieldDecorator('password', {
                rules: [{required: true, message: 'Empty password!'}],
              })(<Input
                prefix={<i
                  className="fas fa-lock"
                  aria-hidden="true"/>}
                type="password"
                placeholder="password"/>)}
            </FormItem>
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

  handleSubmit(e: FormEvent) {
    e.preventDefault();
    this.props.form.validateFields((errors: any, values: any) => {
      if (!errors) {
        fetch('/auth', {
          method: 'POST',
          body: JSON.stringify(values),
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
    });
  }
}

export default withRouter(Form.create<WrappedLoginProps>()(WrappedLogin));
