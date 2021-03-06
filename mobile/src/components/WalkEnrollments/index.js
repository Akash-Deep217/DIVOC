import React, {useState} from "react";
import {Redirect} from "react-router";
import {BaseFormCard} from "../BaseFormCard";
import "./index.scss"
import {Button} from "react-bootstrap";
import {useWalkInEnrollment, WALK_IN_ROUTE, WalkInEnrollmentProvider} from "./context";
import Row from "react-bootstrap/Row";
import PropTypes from 'prop-types';
import schema from '../../jsonSchema/walk_in_form.json';
import Form from "@rjsf/core/lib/components/Form";
import {ImgDirect, ImgGovernment, ImgVoucher} from "../../assets/img/ImageComponents";

export const FORM_WALK_IN_ENROLL_FORM = "form";
export const FORM_WALK_IN_ENROLL_PAYMENTS = "payments";

export function WalkEnrollmentFlow(props) {
    return (
        <WalkInEnrollmentProvider>
            <WalkInEnrollmentRouteCheck pageName={props.match.params.pageName}/>
        </WalkInEnrollmentProvider>
    );
}

function WalkInEnrollmentRouteCheck({pageName}) {
    const {state} = useWalkInEnrollment();
    switch (pageName) {
        case FORM_WALK_IN_ENROLL_FORM :
            return <WalkEnrollment/>;
        case FORM_WALK_IN_ENROLL_PAYMENTS : {
            if (state.name) {
                return <WalkEnrollmentPayment/>
            }
            break;
        }
        default:
    }
    return <Redirect
        to={{
            pathname: '/' + WALK_IN_ROUTE + '/' + FORM_WALK_IN_ENROLL_FORM
        }}
    />
}


function WalkEnrollment(props) {
    const {state, goNext} = useWalkInEnrollment()

    const customFormats = {
        'phone-in': /\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{4}$/
    };

    const uiSchema = {
        classNames: "form-container",
        phone: {
            "ui:placeholder": "+91"
        },
    };
    return (
        <div className="new-enroll-container">
            <BaseFormCard title={"Enroll Recipient"}>
                <div className="pt-3 form-wrapper">
                    <Form
                        schema={schema}
                        customFormats={customFormats}
                        uiSchema={uiSchema}
                        formData={state}
                        onSubmit={(e) => {
                            goNext(FORM_WALK_IN_ENROLL_FORM, FORM_WALK_IN_ENROLL_PAYMENTS, e.formData)
                        }}
                    >
                        <Button type={"submit"} variant="outline-primary" className="action-btn">Done</Button>
                    </Form>
                </div>

            </BaseFormCard>
        </div>
    );
}

const paymentMode = [
    {
        name: "Government",
        logo: function (selected) {
            return <ImgGovernment selected={selected}/>
        }

    }
    ,
    {
        name: "Voucher",
        logo: function (selected) {
            return <ImgVoucher selected={selected}/>
        }

    }
    ,
    {
        name: "Direct",
        logo: function (selected) {
            return <ImgDirect selected={selected}/>
        }

    }
]

function WalkEnrollmentPayment(props) {

    const {goNext, saveWalkInEnrollment} = useWalkInEnrollment()
    const [selectPaymentMode, setSelectPaymentMode] = useState()
    return (
        <div className="new-enroll-container">
            <BaseFormCard title={"Enroll Recipient"}>
                <div className="content">
                    <h3>Please select mode of payment</h3>
                    <Row className="payment-container">
                        {
                            paymentMode.map((item, index) => {
                                return <PaymentItem
                                    title={item.name}
                                    logo={item.logo}
                                    selected={item.name === selectPaymentMode}
                                    onClick={(value) => {
                                        setSelectPaymentMode(value)
                                    }}/>
                            })
                        }
                    </Row>
                    <Button variant="outline-primary"
                            className="action-btn"
                            onClick={() => {
                                saveWalkInEnrollment(selectPaymentMode)
                                    .then(() => {
                                        goNext(FORM_WALK_IN_ENROLL_PAYMENTS, "/", {})
                                    })
                            }}>Send for vaccination</Button>
                </div>
            </BaseFormCard>
        </div>
    );
}


PaymentItem.propTypes = {
    title: PropTypes.string.isRequired,
    logo: PropTypes.object.isRequired,
    selected: PropTypes.bool,
    onClick: PropTypes.func
};

function PaymentItem(props) {
    return (
        <div onClick={() => {
            if (props.onClick) {
                props.onClick(props.title)
            }
        }}>
            <div className={`payment-item ${props.selected ? "active" : ""}`}>
                <div className={"logo"}>
                    {props.logo(props.selected)}
                </div>
                <h6 className="title">{props.title}</h6>
            </div>
        </div>
    );
}
