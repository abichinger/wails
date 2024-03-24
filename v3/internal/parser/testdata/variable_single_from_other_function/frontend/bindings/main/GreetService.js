// @ts-check
// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT

import {Call} from '@wailsio/runtime';
/**
 * @typedef {import('./models').Person} Person
 */

/**
 * Greet does XYZ
 * @function Greet
 * @param name {string}
 * @returns {Promise<string>}
 **/
export async function Greet(name) {
	return Call.ByID(1411160069, ...Array.prototype.slice.call(arguments, 0));
}

/**
 * NewPerson creates a new person
 * @function NewPerson
 * @param name {string}
 * @returns {Promise<Person>}
 **/
export async function NewPerson(name) {
	return Call.ByID(1661412647, ...Array.prototype.slice.call(arguments, 0));
}
