// @ts-check
// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT

export class Person {
    "name": string;

    /** Creates a new Person instance. */
    constructor(source: Partial<Person> = {}) {
        if (!("name" in source)) {
            this["name"] = "";
        }

        Object.assign(this, source);
    }

    /** Creates a new Person instance from a string or object. */
    static createFrom(source: string | object = {}): Person {
        let parsedSource = typeof source === 'string' ? JSON.parse(source) : source;
        return new Person(parsedSource as Partial<Person>);
    }
}
