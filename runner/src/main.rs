use std::{
    env::args,
    fs::read_to_string,
    process::Command,
};

fn main() {
    let file_name = args().nth(1).unwrap();

    let c = read_to_string(file_name).unwrap();

    let mut v = c
        .split('\n')
        .into_iter()
        .map(|x| x.to_owned())
        .collect::<Vec<String>>();
    let mut coms = Vec::new();
    let mut n = Vec::new();
    n.push(v.pop().unwrap());
    n.append(&mut v);

    for c in n {
        if c == "" {
            continue;
        }

        let t = c.split(' ').collect::<Vec<&str>>();

        let c = Command::new(t[0])
            .args(t.into_iter().skip(1))
            .spawn()
            .unwrap();

        coms.push(c);
    }
    for mut c in coms {
        c.wait().unwrap();
    }
}
