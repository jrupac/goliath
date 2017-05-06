import React from 'react';
import { Form, Icon, Input, Button } from 'antd';
const FormItem = Form.Item;


class WrappedLogin extends React.Component {
  render() {
    const { getFieldDecorator } = this.props.form;
    return (
        <div className='login-page'>
          <div className='login-page-logo'>Goliath</div>
          <div className='login-page-form-background'>
            <Form
                onSubmit={this.handleSubmit}
                layout='vertical'
                className='login-page-form'>
              <FormItem>
                {getFieldDecorator('username', {
                  rules: [{ required: true, message: 'Empty username!' }],
                })(<Input
                    prefix={<Icon type="user" style={{ fontSize: 13 }} />}
                    placeholder="username" />)}
              </FormItem>
              <FormItem>
                {getFieldDecorator('password', {
                  rules: [{ required: true, message: 'Empty password!' }],
                })(<Input
                    prefix={<Icon type="lock" style={{ fontSize: 13 }} />}
                    type="password"
                    placeholder="password" />)}
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

  handleSubmit = (e) => {
    e.preventDefault();
    this.props.form.validateFields((err, values) => {
      if (!err) {
        this.setState({isValid: true});
        console.log('Received values of form: ', values);
      }
    });
  };
}

export default Form.create()(WrappedLogin);
